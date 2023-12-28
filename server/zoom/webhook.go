// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"time"
)

type EventType string

const (
	WebhookStatusStarted         = "STARTED"
	WebhookStatusEnded           = "ENDED"
	RecordingWebhookTypeComplete = "RECORDING_MEETING_COMPLETED"
	RecentlyCreated              = "RECENTLY_CREATED"

	EventTypeMeetingStarted              EventType = "meeting.started"
	EventTypeMeetingEnded                EventType = "meeting.ended"
	EventTypeValidateWebhook             EventType = "endpoint.url_validation"
	EventTypeParticipantJoinedBeforeHost EventType = "meeting.participant_jbh_waiting"
)

type MeetingWebhookObject struct {
	Duration  int       `json:"duration"`
	StartTime time.Time `json:"start_time"`
	Timezone  string    `json:"timezone"`
	EndTime   time.Time `json:"end_time"`
	Topic     string    `json:"topic"`
	ID        string    `json:"id"`
	Type      int       `json:"type"`
	UUID      string    `json:"uuid"`
	HostID    string    `json:"host_id"`
}

type MeetingWebhookPayload struct {
	AccountID string               `json:"account_id"`
	Object    MeetingWebhookObject `json:"object"`
}

type MeetingWebhook struct {
	Event   EventType             `json:"event"`
	Payload MeetingWebhookPayload `json:"payload"`
}

type ValidationWebhookPayload struct {
	PlainToken string `json:"plainToken"`
}

type ValidationWebhook struct {
	Event   EventType                `json:"event"`
	Payload ValidationWebhookPayload `json:"payload"`
}

type ValidationWebhookResponse struct {
	PlainToken     string `json:"plainToken"`
	EncryptedToken string `json:"encryptedToken"`
}

type Webhook struct {
	Event     EventType   `json:"event"`
	EventTime int         `json:"event_ts"`
	Payload   interface{} `json:"payload"`
}

type RecordingWebhook struct {
	Type    string `schema:"type"`
	Content string `schema:"content"`
}

type RecordingWebhookContent struct {
	UUID           string    `json:"uuid"`
	MeetingNumber  int       `json:"meeting_number"`
	AccountID      string    `json:"account_id"`
	HostID         string    `json:"host_id"`
	Topic          string    `json:"topic"`
	StartTime      time.Time `json:"start_time"`
	Timezone       string    `json:"timezone"`
	HostEmail      string    `json:"host_email"`
	Duration       int       `json:"duration"`
	TotalSize      int       `json:"total_size"`
	RecordingCount int       `json:"recording_count"`
	RecordingFiles []struct {
		ID             string    `json:"id"`
		MeetingID      string    `json:"meeting_id"`
		RecordingStart time.Time `json:"recording_start"`
		RecordingEnd   time.Time `json:"recording_end"`
		FileType       string    `json:"file_type"`
		FileSize       int       `json:"file_size"`
		FilePath       string    `json:"file_path"`
		Status         string    `json:"status"`
	} `json:"recording_files"`
}

type DeauthorizationEvent struct {
	Event   string
	Payload DeauthorizationPayload
}

type DeauthorizationPayload struct {
	UserDataRetention   string `json:"user_data_retention"`
	AccountID           string `json:"account_id"`
	UserID              string `json:"user_id"`
	Signature           string
	DeauthorizationTime time.Time `json:"deauthorization_time"`
	ClientID            string    `json:"client_id"`
}
