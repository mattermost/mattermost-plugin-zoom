// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

func (p *Plugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if !p.verifyMattermostWebhookSecret(r) {
		p.API.LogWarn("Could not verify Mattermost webhook secret")
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		res := fmt.Sprintf("Expected Content-Type 'application/json' for webhook request, received '%s'.", r.Header.Get("Content-Type"))
		p.API.LogWarn(res)
		http.Error(w, res, http.StatusBadRequest)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		p.API.LogWarn("Cannot read body from Webhook")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var webhook zoom.Webhook
	if err = json.Unmarshal(b, &webhook); err != nil {
		p.API.LogError("Error unmarshalling webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if webhook.Event != zoom.EventTypeValidateWebhook {
		err = p.verifyZoomWebhookSignature(r, b)
		if err != nil {
			p.API.LogWarn("Could not verify webhook signature: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}
	}

	switch webhook.Event {
	case zoom.EventTypeMeetingEnded:
		p.handleMeetingEnded(w, r, b)
	case zoom.EventTypeValidateWebhook:
		p.handleValidateZoomWebhook(w, r, b)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (p *Plugin) handleMeetingEnded(w http.ResponseWriter, _ *http.Request, body []byte) {
	var webhook zoom.MeetingWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		p.client.Log.Error("Error unmarshalling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	meetingPostID := webhook.Payload.Object.ID
	postID, err := p.fetchMeetingPostID(meetingPostID)
	if err != nil {
		return
	}

	post, err := p.client.Post.GetPost(postID)
	if err != nil {
		p.client.Log.Warn("Could not get meeting post by id", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	if err = p.client.Post.UpdatePost(post); err != nil {
		p.client.Log.Warn("Could not update the post", "post_id", postID, "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = p.deleteMeetingPostID(meetingPostID); err != nil {
		p.client.Log.Warn("failed to delete db entry", "err", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.client.Log.Warn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) verifyMattermostWebhookSecret(r *http.Request) bool {
	config := p.getConfiguration()
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) == 1
}

func (p *Plugin) verifyZoomWebhookSignature(r *http.Request, body []byte) error {
	config := p.getConfiguration()
	if config.ZoomWebhookSecret == "" {
		return nil
	}

	var webhook zoom.Webhook
	err := json.Unmarshal(body, &webhook)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling webhook payload")
	}

	ts := r.Header.Get("x-zm-request-timestamp")

	msg := fmt.Sprintf("v0:%s:%s", ts, string(body))
	hash, err := createWebhookSignatureHash(config.ZoomWebhookSecret, msg)
	if err != nil {
		return err
	}

	computedSignature := fmt.Sprintf("v0=%s", hash)
	providedSignature := r.Header.Get("x-zm-signature")
	if computedSignature != providedSignature {
		return errors.New("provided signature does not match")
	}

	return nil
}

func (p *Plugin) handleValidateZoomWebhook(w http.ResponseWriter, _ *http.Request, body []byte) {
	config := p.getConfiguration()
	if config.ZoomWebhookSecret == "" {
		p.API.LogWarn("Failed to validate Zoom webhook: Zoom webhook secret not set")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var webhook zoom.ValidationWebhook
	err := json.Unmarshal(body, &webhook)
	if err != nil {
		p.API.LogWarn("Failed to unmarshal Zoom webhook: " + err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	hash, err := createWebhookSignatureHash(config.ZoomWebhookSecret, webhook.Payload.PlainToken)
	if err != nil {
		p.API.LogWarn("Failed to create webhook signature hash: " + err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	out := zoom.ValidationWebhookResponse{
		PlainToken:     webhook.Payload.PlainToken,
		EncryptedToken: hash,
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func createWebhookSignatureHash(secret, data string) (string, error) {
	h := hmac.New(sha256.New, []byte(secret))
	_, err := h.Write([]byte(data))
	if err != nil {
		return "", errors.Wrap(err, "failed to create webhook signature hash")
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
