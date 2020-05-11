// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
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
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	config := p.getConfiguration()

	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) != 1 {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		res := fmt.Sprintf("Expected Content-Type 'application/json' for webhook request, received '%s'.", r.Header.Get("Content-Type"))
		http.Error(w, res, http.StatusBadRequest)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var webhook zoom.Webhook
	err = json.Unmarshal(b, &webhook)
	if err != nil {
		p.API.LogError("Error unmarshaling webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if webhook.Event != zoom.EventTypeMeetingEnded {
		// TODO: handle recording webhook
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	var meetingWebhook zoom.MeetingWebhook
	err = json.Unmarshal(b, &meetingWebhook)
	if err != nil {
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
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if b == nil {
		http.Error(w, "Stored meeting not found", http.StatusNotFound)
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

	_, appErr = p.API.UpdatePost(post)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	appErr = p.API.KVDelete(key)
	if appErr != nil {
		p.API.LogWarn("failed to delete db entry", "error", appErr.Error())
		return
	}

	_, err := w.Write([]byte(post.ToJson()))
	if err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

type startMeetingRequest struct {
	ChannelID string `json:"channel_id"`
	Personal  bool   `json:"personal"`
	Topic     string `json:"topic"`
	MeetingID int    `json:"meeting_id"`
}

func (p *Plugin) postMeeting(creatorUsername string, meetingID int, channelID string, topic string) (*model.Post, *model.AppError) {

	meetingURL := p.getMeetingURL(meetingID)

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   fmt.Sprintf("Meeting started at %s.", meetingURL),
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"meeting_id":               meetingID,
			"meeting_link":             meetingURL,
			"meeting_status":           zoom.WebhookStatusStarted,
			"meeting_personal":         true,
			"meeting_topic":            topic,
			"meeting_creator_username": creatorUsername,
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
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	_, appErr = p.API.GetChannelMember(req.ChannelID, userID)
	if appErr != nil {
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

	ru, clientErr := p.zoomClient.GetUser(user.Email)
	if clientErr != nil {
		http.Error(w, clientErr.Error(), clientErr.StatusCode)
		return
	}
	meetingID := ru.Pmi

	createdPost, appErr := p.postMeeting(user.Username, meetingID, req.ChannelID, req.Topic)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	appErr = p.API.KVSet(fmt.Sprintf("%v%v", postMeetingKey, meetingID), []byte(createdPost.Id))
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	meetingURL := p.getMeetingURL(meetingID)
	_, err = w.Write([]byte(fmt.Sprintf(`{"meeting_url": "%s"}`, meetingURL)))
	if err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) getMeetingURL(meetingID int) string {
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
