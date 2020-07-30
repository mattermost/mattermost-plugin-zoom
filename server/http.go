// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	defaultMeetingTopic = "Zoom Meeting"
	zoomStateLength     = 2
)

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

	state := fmt.Sprintf("%s_%s", zoomStateKeyPrefix, userID)
	cfg := p.getOAuthConfig()
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)

	http.Redirect(w, r, url, http.StatusFound)
}

func (p *Plugin) completeUserOAuthToZoom(w http.ResponseWriter, r *http.Request) {
	authedUserID := r.Header.Get("Mattermost-User-ID")
	if authedUserID == "" {
		http.Error(w, "Not authorized, missing Mattermost user id", http.StatusUnauthorized)
		return
	}

	code := r.URL.Query().Get("code")
	if len(code) == 0 {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	stateComponents := strings.Split(state, "_")

	if len(stateComponents) != zoomStateLength {
		p.API.LogWarn(fmt.Sprintf("stateComponents: %v, state: %v", stateComponents, state))
		http.Error(w, "invalid state", http.StatusBadRequest)
	}

	userID := stateComponents[1]
	if userID != authedUserID {
		http.Error(w, "Not authorized, incorrect user", http.StatusUnauthorized)
		return
	}

	channelID, err := p.deleteUserState(state)
	if err != nil {
		msg := "failed to delete user state"
		p.API.LogWarn(msg, "error", err.Error())
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	conf := p.getOAuthConfig()
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		p.API.LogWarn("failed to create access token", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client := zoom.NewOAuthClient(token, conf, p.siteURL, p.getZoomAPIURL())
	user, _ := p.API.GetUser(userID)
	zoomUser, authErr := client.GetUser(user)
	if authErr != nil {
		p.API.LogWarn("failed to get user", "error", err.Error())
		http.Error(w, authErr.Error(), http.StatusInternalServerError)
		return
	}

	info := &zoom.OAuthUserInfo{
		ZoomEmail:  zoomUser.Email,
		ZoomID:     zoomUser.ID,
		UserID:     userID,
		OAuthToken: token,
	}

	if err = p.storeOAuthUserInfo(info); err != nil {
		msg := "Unable to connect user to Zoom"
		p.API.LogWarn(msg, "error", err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	if err = p.postMeeting(user, zoomUser.Pmi, channelID, ""); err != nil {
		p.API.LogWarn("Failed to post meeting", "error", err.Error())
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
	meetingPostID := webhook.Payload.Object.ID
	postID, appErr := p.fetchMeetingPostID(meetingPostID)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
	}

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

	if appErr = p.deleteMeetingPostID(meetingPostID); appErr != nil {
		p.API.LogWarn("failed to delete db entry", "error", appErr.Error())
		return
	}

	_, err := w.Write([]byte(post.ToJson()))
	if err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) postMeeting(creator *model.User, meetingID int, channelID string, topic string) error {
	meetingURL := p.getMeetingURL(creator, meetingID, channelID)

	if topic == "" {
		topic = defaultMeetingTopic
	}

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

	if appErr = p.storeMeetingPostID(meetingID, createdPost.Id); appErr != nil {
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

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
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

	meetingURL := p.getMeetingURL(user, meetingID, req.ChannelID)
	_, err = w.Write([]byte(fmt.Sprintf(`{"meeting_url": "%s"}`, meetingURL)))
	if err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) getMeetingURL(user *model.User, meetingID int, channelID string) string {
	defaultURL := fmt.Sprintf("%s/j/%v", p.getZoomURL(), meetingID)
	client, authErr := p.getActiveClient(user)
	if authErr != nil {
		p.API.LogWarn("could not get the active zoom client", "error", authErr.Error())
		return defaultURL
	}

	meeting, err := client.GetMeeting(meetingID)
	if err != nil {
		p.API.LogDebug("failed to get meeting")
		return defaultURL
	}
	return meeting.JoinURL
}

func (p *Plugin) postConfirm(meetingID int, channelID string, topic string, userID string, creatorName string) *model.Post {
	creator, err := p.API.GetUserByUsername(creatorName)
	if err != nil {
		p.API.LogDebug("error fetching user on postConfirm", "error", err.Error())
	}

	meetingURL := p.getMeetingURL(creator, meetingID, channelID)

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

	info, err := p.fetchOAuthUserInfo(zoomTokenKeyByZoomID, req.Payload.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = p.disconnectOAuthUser(info.UserID); err != nil {
		http.Error(w, "Unable to disconnect user from Zoom", http.StatusInternalServerError)
		return
	}

	message := "We have received a deauthorization message from Zoom for your account. We have removed all your Zoom related information from our systems. Please, connect again to Zoom to keep using it."
	if err = p.sendDirectMessage(info.UserID, message); err != nil {
		p.API.LogWarn("failed to dm user about deauthorization", "error", err.Error())
	}

	if req.Payload.UserDataRetention == "false" {
		if err := p.completeCompliance(req.Payload); err != nil {
			p.API.LogWarn("failed to complete compliance after user deauthorization", "error", err.Error())
		}
	}
}

func (p *Plugin) verifyWebhookSecret(r *http.Request) bool {
	config := p.getConfiguration()
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) == 1
}

func (p *Plugin) completeCompliance(payload zoom.DeauthorizationPayload) error {
	data := zoom.ComplianceRequest{
		ClientID:                     payload.ClientID,
		UserID:                       payload.UserID,
		AccountID:                    payload.AccountID,
		DeauthorizationEventReceived: payload,
		ComplianceCompleted:          true,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "could not marshal JSON data")
	}

	res, err := http.Post(
		p.getZoomAPIURL()+"/oauth/data/compliance",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return errors.Wrap(err, "could not make POST request to the data compliance endpoint")
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return errors.Errorf("data compliance request has failed with status code: %d", res.StatusCode)
	}

	return nil
}
