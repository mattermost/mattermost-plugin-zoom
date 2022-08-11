package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

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
		w.WriteHeader(http.StatusOK)
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
		return
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
			"Meeting ID: %d\n\n##### Meeting Summary\n\nDate: %s\n\nMeeting Length: %d minute(s)",
			int(meetingID),
			startText,
			length,
		),
	}

	post.Message = "The meeting has ended."
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) verifyWebhookSecret(r *http.Request) bool {
	config := p.getConfiguration()
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) == 1
}
