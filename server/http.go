// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const defaultMeetingTopic = "Zoom Meeting"

type startMeetingRequest struct {
	ChannelID string `json:"channel_id"`
	Personal  bool   `json:"personal"`
	Topic     string `json:"topic"`
	MeetingID int    `json:"meeting_id"`
}

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
		http.Error(w, fmt.Sprintf("could not get OAuth config: %s", err.Error()), http.StatusInternalServerError)
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

	conf, err := p.getOAuthConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf("could not get OAuth config: %s", err.Error()), http.StatusInternalServerError)
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
	storedState, appErr := p.API.KVGet(key)
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

	if appErr = p.API.KVDelete(state); appErr != nil {
		p.API.LogWarn("failed to delete state from db", "error", appErr.Error())
	}

	if userID != authedUserID {
		http.Error(w, "Not authorized, incorrect user", http.StatusUnauthorized)
		return
	}

	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zoomUser, err := zoom.GetUserViaOAuth(token, conf, p.getZoomAPIURL())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zoomUserInfo := &zoom.UserInfo{
		ZoomEmail:  zoomUser.Email,
		ZoomID:     zoomUser.ID,
		UserID:     userID,
		OAuthToken: token,
	}

	if err = p.storeZoomUserInfo(zoomUserInfo); err != nil {
		http.Error(w, "Unable to connect user to Zoom", http.StatusInternalServerError)
		return
	}

	user, _ := p.API.GetUser(userID)
	if err = p.postMeeting(user, zoomUser.Pmi, channelID, ""); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	if _, err := w.Write([]byte(html)); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if !p.verifyWebhookSecret(r) {
		p.API.LogWarn("Could not verify webhook secreet")
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		res := fmt.Sprintf("Expected Content-Type 'application/json' for webhook request, received '%s'.", r.Header.Get("Content-Type"))
		p.API.LogWarn(res)
		http.Error(w, res, http.StatusBadRequest)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		p.API.LogWarn("Cannot read body from Webhook")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var webhook zoom.Webhook
	if err = json.Unmarshal(b, &webhook); err != nil {
		p.API.LogError("Error unmarshaling webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if webhook.Event != zoom.EventTypeMeetingEnded {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	var meetingWebhook zoom.MeetingWebhook
	if err = json.Unmarshal(b, &meetingWebhook); err != nil {
		p.API.LogError("Error unmarshaling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.handleMeetingEnded(w, r, &meetingWebhook)
}

func (p *Plugin) handleMeetingEnded(w http.ResponseWriter, r *http.Request, webhook *zoom.MeetingWebhook) {
	key := fmt.Sprintf("%v%v", postMeetingKey, webhook.Payload.Object.ID)
	b, appErr := p.API.KVGet(key)
	if appErr != nil {
		p.API.LogDebug("Could not get meeting post from KVStore", "err", appErr.Error())
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if b == nil {
		p.API.LogWarn("Stored meeting not found")
		http.Error(w, "Stored meeting not found", http.StatusNotFound)
		return
	}

	postID := string(b)
	post, appErr := p.API.GetPost(postID)
	if appErr != nil {
		p.API.LogWarn("Could not get meeting post by id", "err", appErr)
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	start := time.Unix(0, post.CreateAt*int64(time.Millisecond))
	length := int(math.Ceil(float64((model.GetMillis()-post.CreateAt)/1000) / 60))
	startText := start.Format("Mon Jan 2 15:04:05 -0700 MST 2006")
	topic, ok := post.Props["meeting_topic"].(string)
	if !ok {
		topic = defaultMeetingTopic
	}

	meetingID, ok := post.Props["meeting_id"].(float64)
	if !ok {
		meetingID = 0
	}

	slackAttachment := model.SlackAttachment{
		Fallback: fmt.Sprintf("Meeting %s has ended: started at %s, length: %d minute(s).", post.Props["meeting_id"], startText, length),
		Title:    topic,
		Text: fmt.Sprintf(
			"Personal Meeting ID (PMI) : %d\n\n##### Meeting Summary\n\nDate: %s\n\nMeeting Length: %d minute(s)",
			int(meetingID),
			startText,
			length,
		),
	}

	post.Message = "I have ended the meeting."
	post.Props["meeting_status"] = zoom.WebhookStatusEnded
	post.Props["attachments"] = []*model.SlackAttachment{&slackAttachment}

	_, appErr = p.API.UpdatePost(post)
	if appErr != nil {
		p.API.LogWarn("Could not update the post", "err", appErr)
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if appErr = p.API.KVDelete(key); appErr != nil {
		p.API.LogWarn("failed to delete db entry", "error", appErr.Error())
		return
	}

	_, err := w.Write([]byte(post.ToJson()))
	if err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) postMeeting(creator *model.User, meetingID int, channelID string, topic string) error {
	if topic == "" {
		topic = defaultMeetingTopic
	}

	meetingURL := p.getMeetingURL(meetingID, creator.Id)

	slackAttachment := model.SlackAttachment{
		Fallback: fmt.Sprintf("Video Meeting started at [%d](%s).\n\n[Join Meeting](%s)", meetingID, meetingURL, meetingURL),
		Title:    topic,
		Text:     fmt.Sprintf("Personal Meeting ID (PMI) : [%d](%s)\n\n[Join Meeting](%s)", meetingID, meetingURL, meetingURL),
	}

	post := &model.Post{
		UserId:    creator.Id,
		ChannelId: channelID,
		Message:   "I have started a meeting",
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"attachments":              []*model.SlackAttachment{&slackAttachment},
			"meeting_id":               meetingID,
			"meeting_link":             meetingURL,
			"meeting_status":           zoom.WebhookStatusStarted,
			"meeting_personal":         true,
			"meeting_topic":            topic,
			"meeting_creator_username": creator.Username,
		},
	}

	createdPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		return appErr
	}

	appErr = p.API.KVSetWithExpiry(fmt.Sprintf("%v%v", postMeetingKey, meetingID), []byte(createdPost.Id), meetingPostIDTTL)
	if appErr != nil {
		p.API.LogDebug("failed to store post id", "err", appErr)
	}

	return nil
}

func (p *Plugin) handleStartMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req startMeetingRequest
	var err error
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	if r.URL.Query().Get("force") == "" {
		recentMeeting, recentMeetindID, creatorName, cpmErr := p.checkPreviousMessages(req.ChannelID)
		if cpmErr != nil {
			http.Error(w, cpmErr.Error(), cpmErr.StatusCode)
			return
		}

		if recentMeeting {
			_, err = w.Write([]byte(`{"meeting_url": ""}`))
			if err != nil {
				p.API.LogWarn("failed to write response", "error", err.Error())
			}
			p.postConfirm(recentMeetindID, req.ChannelID, req.Topic, userID, creatorName)
			return
		}
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(userID, user.Email, req.ChannelID)
	if authErr != nil {
		_, err = w.Write([]byte(`{"meeting_url": ""}`))
		if err != nil {
			p.API.LogWarn("failed to write response", "error", err.Error())
		}
		p.postAuthenticationMessage(req.ChannelID, userID, authErr.Message)
		return
	}

	meetingID := zoomUser.Pmi
	if err = p.postMeeting(user, meetingID, req.ChannelID, req.Topic); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	meetingURL := p.getMeetingURL(meetingID, userID)
	_, err = w.Write([]byte(fmt.Sprintf(`{"meeting_url": "%s"}`, meetingURL)))
	if err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) getMeetingURL(meetingID int, userID string) string {
	defaultURL := fmt.Sprintf("%s/j/%v", p.getZoomURL(), meetingID)

	if !p.getConfiguration().EnableOAuth {
		meeting, err := p.zoomClient.GetMeeting(meetingID)
		if err != nil {
			p.API.LogDebug("failed to get meeting", "error", err.Error())
			return defaultURL
		}
		return meeting.JoinURL
	}

	userInfo, err := p.getZoomUserInfo(userID)
	if err != nil {
		p.API.LogDebug("failed to get Zoom user info", "error", err.Error())
		return defaultURL
	}

	conf, err := p.getOAuthConfig()
	if err != nil {
		p.API.LogDebug("failed to get OAuth config", "error", err.Error())
		return defaultURL
	}

	meeting, err := userInfo.GetMeetingViaOAuth(meetingID, conf, p.getZoomAPIURL())
	if err != nil {
		p.API.LogDebug("failed to get meeting via OAuth", "error", err.Error())
		return defaultURL
	}
	return meeting.JoinURL
}

func (p *Plugin) postConfirm(meetingID int, channelID string, topic string, userID string, creatorName string) *model.Post {
	creator, err := p.API.GetUserByUsername(creatorName)
	if err != nil {
		p.API.LogDebug("error fetching user on postConfirm", "error", err.Error())
	}

	creatorID := ""
	if creator != nil {
		creatorID = creator.Id
	}

	meetingURL := p.getMeetingURL(meetingID, creatorID)

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

func (p *Plugin) postAuthenticationMessage(channelID string, userID string, message string) *model.Post {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   message,
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

	var userInfo zoom.UserInfo
	if err := json.Unmarshal(rawInfo, &userInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := p.disconnect(userInfo.UserID); err != nil {
		http.Error(w, "Unable to disconnect user from Zoom", http.StatusInternalServerError)
		return
	}

	err := p.sendDirectMessage(userInfo.UserID, "We have received a deauthorization message from Zoom for your account. We have removed all your Zoom related information from our systems. Please, connect again to Zoom to keep using it.")
	if err != nil {
		p.API.LogWarn("failed to dm user about deauthorization", "error", err.Error())
	}

	if req.Payload.UserDataRetention == "true" {
		if err := p.zoomClient.CompleteCompliance(req.Payload); err != nil {
			p.API.LogWarn("failed to complete compliance after user deauthorization", "error", err.Error())
		}
	}
}

func (p *Plugin) verifyWebhookSecret(r *http.Request) bool {
	config := p.getConfiguration()
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) == 1
}
