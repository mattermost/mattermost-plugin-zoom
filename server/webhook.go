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
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const bearerString = "Bearer "
const maxWebhookBodySize = 1 << 20 // 1MB

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

	b, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize+1))
	if err != nil {
		p.API.LogWarn("Cannot read body from Webhook")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(b)) > maxWebhookBodySize {
		p.API.LogWarn("Webhook request body too large")
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
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
	case zoom.EventTypeMeetingStarted:
		p.handleMeetingStarted(w, r, b)
	case zoom.EventTypeMeetingEnded:
		p.handleMeetingEnded(w, r, b)
	case zoom.EventTypeValidateWebhook:
		p.handleValidateZoomWebhook(w, r, b)
	case zoom.EventTypeRecordingCompleted:
		p.handleRecordingCompleted(w, r, b)
	case zoom.EventTypeTranscriptCompleted:
		p.handleTranscriptCompleted(w, r, b)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (p *Plugin) handleMeetingStarted(w http.ResponseWriter, r *http.Request, body []byte) {
	var webhook zoom.MeetingWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		p.API.LogError("Error unmarshaling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	meetingID, err := strconv.Atoi(webhook.Payload.Object.ID)
	if err != nil {
		p.API.LogError("Failed to get meeting ID", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entry, appErr := p.getMeetingChannelEntry(meetingID)
	if appErr != nil || entry == nil || entry.ChannelID == "" {
		return
	}

	channelID := entry.ChannelID

	// For ad-hoc meetings (started via /zoom start), a post already exists.
	// Don't create a duplicate — just update the stored UUID mapping so that
	// meeting.ended can find the post later.
	// Subscription meetings should always create a new post.
	if !entry.IsSubscription {
		if existingPostID, err := p.findMeetingPostByMeetingID(meetingID); err == nil {
			if appErr := p.storeMeetingPostID(webhook.Payload.Object.UUID, existingPostID); appErr != nil {
				p.API.LogWarn("failed to store UUID mapping for existing post",
					"error", appErr.Error(),
				)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	botUser, appErr := p.API.GetUser(p.botUserID)
	if appErr != nil {
		p.API.LogError("Failed to get bot user", "err", appErr.Error())
		return
	}

	if postMeetingErr := p.postMeeting(botUser, meetingID, webhook.Payload.Object.UUID, channelID, "", webhook.Payload.Object.Topic); postMeetingErr != nil {
		p.API.LogError("Failed to post the zoom message in the channel", "err", postMeetingErr.Error())
		return
	}

	p.trackMeetingStart(p.botUserID, telemetryStartSourceSubscribeWebhook)
	p.trackMeetingType(p.botUserID, false)
}

func (p *Plugin) handleMeetingEnded(w http.ResponseWriter, r *http.Request, body []byte) {
	var webhook zoom.MeetingWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		p.client.Log.Error("Error unmarshalling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	webhookMeetingID := webhook.Payload.Object.ID
	webhookUUID := webhook.Payload.Object.UUID

	postID, err := p.fetchMeetingPostID(webhookUUID)
	if err != nil {
		// The UUID Zoom sends at meeting.ended can differ from the one at creation
		// (e.g. PMI meetings, or recurring meetings get a new UUID per occurrence).
		// Fall back to finding the post via the meeting_channel mapping and recent posts.
		meetingIDInt, atoiErr := strconv.Atoi(webhookMeetingID)
		if atoiErr != nil {
			http.Error(w, "meeting post not found", http.StatusNotFound)
			return
		}

		postID, err = p.findMeetingPostByMeetingID(meetingIDInt)
		if err != nil {
			p.API.LogWarn("could not find meeting post",
				"meeting_id", meetingIDInt,
				"error", err.Error(),
			)
			http.Error(w, "meeting post not found", http.StatusNotFound)
			return
		}
	}

	post, err := p.client.Post.GetPost(postID)
	if err != nil {
		p.client.Log.Warn("Could not get meeting post by id", "post_id", postID, "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if post.Props["meeting_status"] == zoom.WebhookStatusEnded {
		w.WriteHeader(http.StatusOK)
		return
	}

	start := time.Unix(0, post.CreateAt*int64(time.Millisecond))
	end := model.GetMillis()
	length := int(math.Ceil(float64((end-post.CreateAt)/1000) / 60))
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
	post.Props["meeting_end_time"] = end
	post.Props["attachments"] = []*model.SlackAttachment{&slackAttachment}

	if err = p.client.Post.UpdatePost(post); err != nil {
		p.client.Log.Warn("Could not update the post", "post_id", postID, "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// NOTE: We intentionally do NOT delete the meeting_channel mapping here.
	// Recording and transcript webhooks arrive after meeting.ended and need
	// the mapping to locate the post. The entry is small and gets overwritten
	// if the same meeting ID is reused.

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) findMeetingPostByMeetingID(meetingID int) (string, error) {
	return p.findMeetingPostByMeetingIDWithFilter(meetingID, true)
}

// findMeetingPostByMeetingIDWithFilter searches recent posts in the channel
// associated with meetingID for a custom_zoom post matching that ID.
// When activeOnly is true, posts already marked as ENDED are skipped.
func (p *Plugin) findMeetingPostByMeetingIDWithFilter(meetingID int, activeOnly bool) (string, error) {
	channelID, appErr := p.fetchChannelForMeeting(meetingID)
	if appErr != nil || channelID == "" {
		return "", errors.Errorf("no channel found for meeting %d", meetingID)
	}

	since := model.GetMillis() - meetingPostIDTTL*1000
	postList, appErr := p.API.GetPostsSince(channelID, since)
	if appErr != nil {
		return "", errors.Wrap(appErr, "could not get recent posts for channel")
	}

	// Prefer the most recently created matching post.
	var bestPostID string
	var bestCreateAt int64
	for _, post := range postList.Posts {
		if post.Type != "custom_zoom" {
			continue
		}
		propID, ok := post.Props["meeting_id"].(float64)
		if !ok || int(propID) != meetingID {
			continue
		}
		if activeOnly && post.Props["meeting_status"] == zoom.WebhookStatusEnded {
			continue
		}
		if post.CreateAt > bestCreateAt {
			bestPostID = post.Id
			bestCreateAt = post.CreateAt
		}
	}

	if bestPostID != "" {
		return bestPostID, nil
	}

	return "", errors.Errorf("no meeting post found for meeting %d in channel %s (active_only=%v)", meetingID, channelID, activeOnly)
}

// resolveRecordingMeetingPost finds the meeting post by UUID first, falling
// back to a meeting-ID-based search when the UUID doesn't match (PMI /
// recurring meetings get a new UUID per occurrence).
func (p *Plugin) resolveRecordingMeetingPost(webhookUUID string, meetingID int) (*model.Post, error) {
	postID, err := p.fetchMeetingPostID(webhookUUID)
	if err != nil {
		// Recording/transcript webhooks arrive after meeting.ended, so the post
		// is already marked ENDED. Use activeOnly=false to include ended posts.
		postID, err = p.findMeetingPostByMeetingIDWithFilter(meetingID, false)
		if err != nil {
			return nil, errors.Wrapf(err, "could not find meeting post for uuid=%s meeting_id=%d", webhookUUID, meetingID)
		}
	}

	post, getErr := p.client.Post.GetPost(postID)
	if getErr != nil {
		return nil, errors.Wrap(getErr, "could not get meeting post by id")
	}

	return post, nil
}

func (p *Plugin) handleTranscript(recording zoom.RecordingFile, postID, channelID, downloadToken string) error {
	request, err := http.NewRequest(http.MethodGet, recording.DownloadURL, nil)
	if err != nil {
		p.API.LogWarn("Unable to get the transcription", "err", err)
		return err
	}
	request.Header.Set("Authorization", bearerString+downloadToken)

	retries := 5
	var response *http.Response
	for retries > 0 {
		response, err = http.DefaultClient.Do(request)
		if err != nil {
			p.API.LogWarn("Unable to get the transcription", "err", err)
			time.Sleep(1 * time.Second)
			retries--
			continue
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			p.API.LogWarn("Unable to get the transcription", "err", "bad status code "+strconv.Itoa(response.StatusCode))
			time.Sleep(1 * time.Second)
			retries--
			continue
		}
		break
	}

	if response == nil {
		p.API.LogWarn("Unable to get the transcription", "err", "response is nil")
		return err
	}

	defer response.Body.Close()
	transcriptionBytes, err := io.ReadAll(response.Body)
	if err != nil {
		p.API.LogWarn("Unable to get the transcription", "err", err)
		return err
	}
	fileInfo, appErr := p.API.UploadFile(transcriptionBytes, channelID, "transcription.txt")
	if appErr != nil {
		p.API.LogWarn("Unable to save transcription file to the channel", "err", appErr)
		return appErr
	}
	newPost := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		RootId:    postID,
		Message:   "Here's the zoom meeting transcription",
		FileIds:   []string{},
		Type:      "custom_zoom_transcript",
	}

	newPost.FileIds = append(newPost.FileIds, fileInfo.Id)
	newPost.AddProp("captions", []any{map[string]any{"file_id": fileInfo.Id}})

	_, appErr = p.API.CreatePost(newPost)
	if appErr != nil {
		p.API.LogWarn("Could not update the post", "err", appErr)
		return appErr
	}

	return nil
}

func (p *Plugin) handleTranscriptCompleted(w http.ResponseWriter, r *http.Request, body []byte) {
	var webhook zoom.RecordingWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		p.API.LogError("Error unmarshaling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	post, err := p.resolveRecordingMeetingPost(webhook.Payload.Object.UUID, webhook.Payload.Object.ID)
	if err != nil {
		p.API.LogWarn("Could not resolve meeting post for transcript", "error", err.Error())
		http.Error(w, "meeting post not found", http.StatusNotFound)
		return
	}

	lastTranscriptionIdx := -1
	for idx, recording := range webhook.Payload.Object.RecordingFiles {
		if recording.RecordingType == zoom.RecordingTypeAudioTranscript {
			lastTranscriptionIdx = idx
		}
	}

	if lastTranscriptionIdx != -1 {
		err := p.handleTranscript(webhook.Payload.Object.RecordingFiles[lastTranscriptionIdx], post.Id, post.ChannelId, webhook.DownloadToken)
		if err != nil {
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) handleRecordingCompleted(w http.ResponseWriter, r *http.Request, body []byte) {

	var webhook zoom.RecordingWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		p.API.LogError("handleRecordingCompleted: failed to unmarshal", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	post, err := p.resolveRecordingMeetingPost(webhook.Payload.Object.UUID, webhook.Payload.Object.ID)
	if err != nil {
		p.API.LogWarn("handleRecordingCompleted: could not resolve meeting post", "error", err.Error())
		http.Error(w, "meeting post not found", http.StatusNotFound)
		return
	}

	recordings := make(map[time.Time][]zoom.RecordingFile)

	for _, recording := range webhook.Payload.Object.RecordingFiles {
		switch {
		case recording.RecordingType == zoom.RecordingTypeChat:
			recordings[recording.RecordingStart] = append(recordings[recording.RecordingStart], recording)
		case strings.EqualFold(recording.FileType, zoom.RecordingFileTypeMP4):
			recordings[recording.RecordingStart] = append(recordings[recording.RecordingStart], recording)
		}
	}

	for _, recordingGroup := range recordings {
		newPost := &model.Post{
			UserId:    p.botUserID,
			ChannelId: post.ChannelId,
			RootId:    post.Id,
			Message:   "",
			FileIds:   []string{},
		}
		for _, recording := range recordingGroup {
			if recording.RecordingType == zoom.RecordingTypeChat {
				fileInfo, chatErr := p.downloadAndUploadChat(recording, webhook.DownloadToken, post.ChannelId)
				if chatErr != nil {
					p.API.LogWarn("handleRecordingCompleted: failed to download/upload chat", "error", chatErr.Error())
					return
				}

				newPost.FileIds = append(newPost.FileIds, fileInfo.Id)
				newPost.AddProp("captions", []any{map[string]any{"file_id": fileInfo.Id}})
				newPost.Type = "custom_zoom_chat"
			} else if strings.EqualFold(recording.FileType, zoom.RecordingFileTypeMP4) && recording.PlayURL != "" {
				msg := "Here's the zoom meeting recording:\n**Link:** [Meeting Recording](" + recording.PlayURL + ")"
				if webhook.Payload.Object.Password != "" {
					msg += "\n**Password:** `" + webhook.Payload.Object.Password + "`"
				}
				newPost.Message = msg
			}
		}

		if newPost.Message != "" {
			_, appErr := p.API.CreatePost(newPost)
			if appErr != nil {
				p.API.LogWarn("handleRecordingCompleted: could not create post", "err", appErr)
				return
			}
		} else if len(newPost.FileIds) > 0 {
			_, appErr := p.API.CreatePost(newPost)
			if appErr != nil {
				p.API.LogWarn("handleRecordingCompleted: could not create chat post", "err", appErr)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) downloadAndUploadChat(recording zoom.RecordingFile, downloadToken, channelID string) (*model.FileInfo, error) {
	request, err := http.NewRequest(http.MethodGet, recording.DownloadURL, nil)
	if err != nil {
		p.API.LogWarn("Unable to get the chat", "err", err)
		return nil, err
	}
	request.Header.Set("Authorization", bearerString+downloadToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		p.API.LogWarn("Unable to get the chat", "err", err)
		return nil, err
	}
	defer response.Body.Close()

	chat, err := io.ReadAll(response.Body)
	if err != nil {
		p.API.LogWarn("Unable to get the chat", "err", err)
		return nil, err
	}

	fileInfo, appErr := p.API.UploadFile(chat, channelID, "Chat-history.txt")
	if appErr != nil {
		p.API.LogWarn("Unable to upload the chat file", "err", appErr)
		return nil, appErr
	}

	return fileInfo, nil
}

func (p *Plugin) verifyMattermostWebhookSecret(r *http.Request) bool {
	config := p.getConfiguration()
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) == 1
}

const webhookTimestampMaxAge = 5 * time.Minute

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

	// Validate timestamp to prevent replay attacks
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("invalid timestamp format")
	}

	requestTime := time.Unix(tsInt, 0)
	timeDiff := time.Since(requestTime)
	if timeDiff > webhookTimestampMaxAge {
		return errors.New("webhook timestamp is too old")
	}
	if timeDiff < -webhookTimestampMaxAge {
		return errors.New("webhook timestamp is too far in the future")
	}

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
