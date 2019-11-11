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

	"github.com/gorilla/schema"
	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
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
	case "/oauth/connect":
		p.connectUserToZoom(w, r)
	case "/oauth/complete":
		p.completeUserOAuthToZoom(w, r)
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
		http.Error(w, "Not authorized", http.StatusUnauthorized)
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
	log.Println("redirect url is", url)
	http.Redirect(w, r, url, http.StatusFound)
}

func (p *Plugin) completeUserOAuthToZoom(w http.ResponseWriter, r *http.Request) {
	authedUserID := r.Header.Get("Mattermost-User-ID")
	if authedUserID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
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

	if len(stateComponents) != 3 {
		log.Printf("stateComponents: %v, state: %v", stateComponents, state)
		http.Error(w, "invalid state", http.StatusBadRequest)

	}
	key := fmt.Sprintf("%v_%v", stateComponents[0], stateComponents[1])

	if storedState, err := p.API.KVGet(key); err != nil {
		fmt.Println(err.Error())
		http.Error(w, "missing stored state", http.StatusBadRequest)
		return
	} else if string(storedState) != state {
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
		fmt.Println(err.Error())
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
		UserID:     userID,
		OAuthToken: tok,
	}

	if err := p.storeZoomUserInfo(zoomUserInfo); err != nil {
		fmt.Println(err.Error())
		http.Error(w, "Unable to connect user to Zoom", http.StatusInternalServerError)
		return
	}

	if err := p.storeZoomToUserIDMapping(zoomUser.Email, userID); err != nil {
		fmt.Println(err.Error())
	}

	_, appErr := p.postMeeting(zoomUser.Pmi, channelID, "")
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
	config := p.getConfiguration()

	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) != 1 {
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

func (p *Plugin) postMeeting(meetingID int, channelID string, topic string) (*model.Post, *model.AppError) {

	config := p.getConfiguration()
	zoomURL := strings.TrimSpace(config.ZoomURL)
	if len(zoomURL) == 0 {
		zoomURL = "https://zoom.us"
	}

	meetingURL := fmt.Sprintf("%s/j/%v", zoomURL, meetingID)

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   fmt.Sprintf("Meeting started at %s.", meetingURL),
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"meeting_id":       meetingID,
			"meeting_link":     meetingURL,
			"meeting_status":   zoom.WebhookStatusStarted,
			"meeting_personal": true,
			"meeting_topic":    topic,
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

	zoomUser, authErr := p.authenticateAndFetchZoomUser(userID, user.Email, req.ChannelID)
	if authErr != nil {
		http.Error(w, authErr.Error(), http.StatusInternalServerError)
		return
	}

	meetingID := zoomUser.Pmi

	createdPost, appErr := p.postMeeting(meetingID, req.ChannelID, req.Topic)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if appErr = p.API.KVSet(fmt.Sprintf("%v%v", postMeetingKey, meetingID), []byte(createdPost.Id)); appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if _, err := w.Write([]byte(fmt.Sprintf("%v", meetingID))); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}
