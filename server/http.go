// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/schema"
	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"golang.org/x/oauth2"
)

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		http.Error(w, "This plugin is not configured.", http.StatusNotImplemented)
		return
	}

	switch path := r.URL.Path; path {
	case "/webhook":
		p.handleWebhook(w, r)
	case "/api/v1/meetings":
		p.handleStartMeeting(w, r)
	case "/oauth2/connect":
		p.connectUserToZoom(w, r)
	case "/oauth2/complete":
		p.completeUserOAuthToZoom(w, r)
	case "/deauthorization":
		p.deauthorizeUser(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) connectUserToZoom(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	channelID := r.URL.Query().Get("channelID")
	if channelID == "" {
		http.Error(w, "channelID missing", http.StatusBadRequest)
		return
	}

	conf, err := p.getOAuthConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	key := fmt.Sprintf("%v_%v", model.NewId()[0:15], userID)
	state := fmt.Sprintf("%v_%v", key, channelID)

	appErr := p.API.KVSet(key, []byte(state))
	if appErr != nil {
		http.Error(w, appErr.Error(), http.StatusInternalServerError)
	}

	url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (p *Plugin) completeUserOAuthToZoom(w http.ResponseWriter, r *http.Request) {
	authedUserID := r.Header.Get("Mattermost-User-ID")
	if authedUserID == "" {
		http.Error(w, "Not authorized, missing Mattermost user id", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	conf, err := p.getOAuthConfig()
	if err != nil {
		http.Error(w, "error in oauth config", http.StatusInternalServerError)
	}

	code := r.URL.Query().Get("code")
	if len(code) == 0 {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	stateComponents := strings.Split(state, "_")

	if len(stateComponents) != zoomStateLength {
		log.Printf("stateComponents: %v, state: %v", stateComponents, state)
		http.Error(w, "invalid state", http.StatusBadRequest)

	}
	key := fmt.Sprintf("%v_%v", stateComponents[0], stateComponents[1])

	var storedState []byte
	var appErr *model.AppError
	storedState, appErr = p.API.KVGet(key)
	if appErr != nil {
		fmt.Println(appErr)
		http.Error(w, "missing stored state", http.StatusBadRequest)
		return
	}

	if string(storedState) != state {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	userID := stateComponents[1]
	channelID := stateComponents[2]

	p.API.KVDelete(state)

	if userID != authedUserID {
		http.Error(w, "Not authorized, incorrect user", http.StatusUnauthorized)
		return
	}

	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zoomUser, err := p.getZoomUserWithToken(tok)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zoomUserInfo := &ZoomUserInfo{
		ZoomEmail:  zoomUser.Email,
		ZoomID:     zoomUser.ID,
		UserID:     userID,
		OAuthToken: tok,
	}

	if err := p.storeZoomUserInfo(zoomUserInfo); err != nil {
		http.Error(w, "Unable to connect user to Zoom", http.StatusInternalServerError)
		return
	}

	user, _ := p.API.GetUser(userID)

	_, appErr = p.postMeeting(user.Username, zoomUser.Pmi, channelID, "")
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	html := `
<!DOCTYPE html>
<html>
	<head>
		<script>
			window.close();
		</script>
	</head>
	<body>
		<p>Completed connecting to Zoom. Please close this window.</p>
	</body>
</html>
`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (p *Plugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if !p.verifyWebhookSecret(r) {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}

	var webhook zoom.Webhook
	decoder := schema.NewDecoder()

	// Try to decode to standard webhook
	if err := decoder.Decode(&webhook, r.PostForm); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.handleStandardWebhook(w, r, &webhook)

	// TODO: handle recording webhook
}

func (p *Plugin) handleStandardWebhook(w http.ResponseWriter, r *http.Request, webhook *zoom.Webhook) {
	if webhook.Status != zoom.WebhookStatusEnded {
		return
	}

	key := fmt.Sprintf("%v%v", postMeetingKey, webhook.ID)
	b, appErr := p.API.KVGet(key)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if b == nil {
		return
	}
	postID := string(b)

	post, appErr := p.API.GetPost(postID)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	post.Message = "Meeting has ended."
	post.Props["meeting_status"] = zoom.WebhookStatusEnded

	if _, appErr := p.API.UpdatePost(post); appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if appErr := p.API.KVDelete(key); appErr != nil {
		p.API.LogWarn("failed to delete db entry", "error", appErr.Error())
		return
	}

	if _, err := w.Write([]byte(post.ToJson())); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

type startMeetingRequest struct {
	ChannelID string `json:"channel_id"`
	Personal  bool   `json:"personal"`
	Topic     string `json:"topic"`
	MeetingID int    `json:"meeting_id"`
}

func (p *Plugin) postMeeting(creator *model.User, meetingID int, channelID string, topic string) (*model.Post, *model.AppError) {

	meetingURL := p.getMeetingURL(meetingID)

	post := &model.Post{
		UserId:    creator.Id,
		ChannelId: channelID,
		Message:   fmt.Sprintf("Meeting started at %s.", meetingURL),
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"meeting_id":               meetingID,
			"meeting_link":             meetingURL,
			"meeting_status":           zoom.WebhookStatusStarted,
			"meeting_personal":         true,
			"meeting_topic":            topic,
			"meeting_creator_username": creator.Username,
		},
	}

	return p.API.CreatePost(post)
}

func (p *Plugin) handleStartMeeting(w http.ResponseWriter, r *http.Request) {

	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req startMeetingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if _, appErr = p.API.GetChannelMember(req.ChannelID, userID); appErr != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if forceCreate := r.URL.Query().Get("force"); forceCreate == "" {
		recentMeeting, recentMeetindID, creatorName, cpmErr := p.checkPreviousMessages(req.ChannelID)
		if cpmErr != nil {
			http.Error(w, cpmErr.Error(), cpmErr.StatusCode)
			return
		}

		if recentMeeting {
			if _, err := w.Write([]byte(`{"meeting_url": ""}`)); err != nil {
				p.API.LogWarn("failed to write response", "error", err.Error())
			}
			p.postConfirm(recentMeetindID, req.ChannelID, req.Topic, userID, creatorName)
			return
		}
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(userID, user.Email, req.ChannelID)
	if authErr != nil {
		if _, err := w.Write([]byte(`{"meeting_url": ""}`)); err != nil {
			p.API.LogWarn("failed to write response", "error", err.Error())
		}
		p.postConnect(req.ChannelID, userID)
		return
	}

	meetingID := zoomUser.Pmi

	createdPost, appErr := p.postMeeting(user, meetingID, req.ChannelID, req.Topic)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if appErr = p.API.KVSet(fmt.Sprintf("%v%v", postMeetingKey, meetingID), []byte(createdPost.Id)); appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	meetingURL := p.getMeetingURL(meetingID)

	if _, err := w.Write([]byte(fmt.Sprintf(`{"meeting_url": "%s"}`, meetingURL))); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) getMeetingURL(meetingID int) string {
	meeting, err := p.zoomClient.GetMeeting(meetingID)
	if err == nil {
		return meeting.JoinURL
	}

	config := p.getConfiguration()
	zoomURL := strings.TrimSpace(config.ZoomURL)
	if len(zoomURL) == 0 {
		zoomURL = "https://zoom.us"
	}

	return fmt.Sprintf("%s/j/%v", zoomURL, meetingID)
}

func (p *Plugin) postConfirm(meetingID int, channelID string, topic string, userID string, creatorName string) *model.Post {
	meetingURL := p.getMeetingURL(meetingID)

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   "There is another recent meeting created on this channel.",
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"type":                     "custom_zoom",
			"meeting_id":               meetingID,
			"meeting_link":             meetingURL,
			"meeting_status":           zoom.RecentlyCreated,
			"meeting_personal":         true,
			"meeting_topic":            topic,
			"meeting_creator_username": creatorName,
		},
	}

	return p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) postConnect(channelID string, userID string) *model.Post {
	oauthMsg := fmt.Sprintf(
		zoomOAuthMessage,
		*p.API.GetConfig().ServiceSettings.SiteURL, channelID)

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   oauthMsg,
	}

	return p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) checkPreviousMessages(channelID string) (recentMeeting bool, meetindID int, creatorName string, err *model.AppError) {
	var zoomMeetingTimeWindow int64 = 30 // 30 seconds

	postList, appErr := p.API.GetPostsSince(channelID, (time.Now().Unix()-zoomMeetingTimeWindow)*1000)
	if appErr != nil {
		return false, 0, "", appErr
	}

	for _, post := range postList.ToSlice() {
		if meetingID, ok := post.Props["meeting_id"]; ok {
			return true, int(meetingID.(float64)), post.Props["meeting_creator_username"].(string), nil
		}
	}

	return false, 0, "", nil
}

func (p *Plugin) deauthorizeUser(w http.ResponseWriter, r *http.Request) {
	if !p.verifyWebhookSecret(r) {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req zoom.DeauthorizationEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rawInfo, appErr := p.API.KVGet(zoomTokenKeyByZoomID + req.Payload.UserID)
	if appErr != nil {
		http.Error(w, appErr.Error(), http.StatusInternalServerError)
		return
	}

	var info ZoomUserInfo
	err := json.Unmarshal(rawInfo, &info)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.disconnect(info.UserID)

	p.dm(info.UserID, "We have received a deauthorization message from Zoom for your account. We have removed all your Zoom related information from our systems. Please, connect again to Zoom to keep using it.")

	if req.Payload.UserDataRetention == "true" {
		p.zoomClient.CompleteCompliance(req.Payload)
	}
}

func (p *Plugin) verifyWebhookSecret(r *http.Request) bool {
	config := p.getConfiguration()

	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) != 1 {
		return false
	}

	return true
}
