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
		p.API.LogError("Error unmarshaling webhook", "err", err.Error())
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

	channelID, appErr := p.fetchChannelForMeeting(meetingID)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if channelID == "" {
		return
	}

	botUser, appErr := p.API.GetUser(p.botUserID)
	if appErr != nil {
		p.API.LogError("Failed to get bot user", "err", appErr.Error())
		http.Error(w, appErr.Error(), http.StatusBadRequest)
		return
	}

	if postMeetingErr := p.postMeeting(botUser, meetingID, webhook.Payload.Object.UUID, channelID, "", webhook.Payload.Object.ID); postMeetingErr != nil {
		p.API.LogError("Failed to post the zoom message in the channel", "err", postMeetingErr.Error())
		http.Error(w, postMeetingErr.Error(), http.StatusBadRequest)
		return
	}

	p.trackMeetingStart(p.botUserID, telemetryStartSourceCommand)
	p.trackMeetingType(p.botUserID, false)
}

func (p *Plugin) handleMeetingEnded(w http.ResponseWriter, r *http.Request, body []byte) {
	var webhook zoom.MeetingWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		p.API.LogError("Error unmarshaling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	meetingPostID := webhook.Payload.Object.UUID
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}

func (p *Plugin) handleTranscript(recording zoom.RecordingFile, postID, channelID, downloadToken string) error {
	request, err := http.NewRequest(http.MethodGet, recording.DownloadURL, nil)
	if err != nil {
		p.API.LogWarn("Unable to get the transcription", "err", err)
		return err
	}
	request.Header.Set("Authorization", "Bearer "+downloadToken)

	retries := 5
	var response *http.Response
	for retries > 0 {
		var err error
		response, err = http.DefaultClient.Do(request)
		if err != nil {
			p.API.LogWarn("Unable to get the transcription", "err", err)
			time.Sleep(1 * time.Second)
			retries -= 1
			continue
		}
		if response.StatusCode != http.StatusOK {
			p.API.LogWarn("Unable to get the transcription", "err", "bad status code "+strconv.Itoa(response.StatusCode))
			time.Sleep(1 * time.Second)
			retries -= 1
			continue
		}
		break
	}

	if response == nil {
		p.API.LogWarn("Unable to get the transcription", "err", "response is nil")
		return err
	}

	defer response.Body.Close()
	transcription, err := io.ReadAll(response.Body)
	if err != nil {
		p.API.LogWarn("Unable to get the transcription", "err", err)
		return err
	}
	fileInfo, appErr := p.API.UploadFile(transcription, channelID, "transcription.txt")
	if appErr != nil {
		p.API.LogWarn("Unable to get the transcription", "err", appErr)
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

	meetingPostID := webhook.Payload.Object.UUID
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

	var lastTranscription *zoom.RecordingFile
	for _, recording := range webhook.Payload.Object.RecordingFiles {
		if recording.RecordingType == zoom.RecordingTypeAudioTranscript {
			lastTranscription = &recording
		}
	}

	if lastTranscription != nil {
		err := p.handleTranscript(*lastTranscription, post.Id, post.ChannelId, webhook.DownloadToken)
		if err != nil {
			http.Error(w, appErr.Error(), appErr.StatusCode)
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
		p.API.LogError("Error unmarshaling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	meetingPostID := webhook.Payload.Object.UUID
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

	recordings := make(map[time.Time][]zoom.RecordingFile)

	for _, recording := range webhook.Payload.Object.RecordingFiles {
		if recording.RecordingType == zoom.RecordingTypeChat {
			recordings[recording.RecordingStart] = append(recordings[recording.RecordingStart], recording)
		}

		if recording.RecordingType == zoom.RecordingTypeVideo {
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
				request, err := http.NewRequest(http.MethodGet, recording.DownloadURL, nil)
				if err != nil {
					p.API.LogWarn("Unable to get the chat", "err", err)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				request.Header.Set("Authorization", "Bearer "+webhook.DownloadToken)
				response, err := http.DefaultClient.Do(request)
				if err != nil {
					p.API.LogWarn("Unable to get the chat", "err", err)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				defer response.Body.Close()
				chat, err := io.ReadAll(response.Body)
				if err != nil {
					p.API.LogWarn("Unable to get the chat", "err", err)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				fileInfo, appErr := p.API.UploadFile(chat, post.ChannelId, "Chat-history.txt")
				if appErr != nil {
					p.API.LogWarn("Unable to get the chat", "err", appErr)
					http.Error(w, appErr.Error(), http.StatusBadRequest)
					return
				}

				newPost.FileIds = append(newPost.FileIds, fileInfo.Id)
				newPost.AddProp("captions", []any{map[string]any{"file_id": fileInfo.Id}})
				newPost.Type = "custom_zoom_chat"
				if newPost.Message == "" {
					newPost.Message = " "
				}
			}
			if recording.RecordingType == zoom.RecordingTypeVideo {
				newPost.Message = "Here's the zoom meeting recording:\n**Link:** [Meeting Recording](" + recording.PlayURL + ")\n**Password:** " + webhook.Payload.Object.Password
			}
		}
		if newPost.Message != "" {
			_, appErr = p.API.CreatePost(newPost)
			if appErr != nil {
				p.API.LogWarn("Could not update the post", "err", appErr)
				http.Error(w, appErr.Error(), appErr.StatusCode)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
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
		return errors.Wrap(err, "error unmarshaling webhook payload")
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

func (p *Plugin) handleValidateZoomWebhook(w http.ResponseWriter, r *http.Request, body []byte) {
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
