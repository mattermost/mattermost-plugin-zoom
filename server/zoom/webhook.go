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

	EventTypeMeetingStarted EventType = "meeting.started"
	EventTypeMeetingEnded   EventType = "meeting.ended"
)

type MeetingWebhookObject struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Timezone  string    `json:"timezone"`
	Topic     string    `json:"topic"`
	ID        string    `json:"id"`
	UUID      string    `json:"uuid"`
	HostID    string    `json:"host_id"`
	Type      int       `json:"type"`
	Duration  int       `json:"duration"`
}

type MeetingWebhookPayload struct {
	AccountID string               `json:"account_id"`
	Object    MeetingWebhookObject `json:"object"`
}

type MeetingWebhook struct {
	Event   EventType             `json:"event"`
	Payload MeetingWebhookPayload `json:"payload"`
}

type Webhook struct {
	Payload interface{} `json:"payload"`
	Event   EventType   `json:"event"`
}

type RecordingWebhook struct {
	Type    string `schema:"type"`
	Content string `schema:"content"`
}

type RecordingWebhookContent struct {
	StartTime time.Time `json:"start_time"`
	UUID      string    `json:"uuid"`
	AccountID string    `json:"account_id"`
	HostID    string    `json:"host_id"`
	Topic     string    `json:"topic"`
	Timezone  string    `json:"timezone"`
	HostEmail string    `json:"host_email"`

	RecordingFiles []struct {
		RecordingStart time.Time `json:"recording_start"`
		RecordingEnd   time.Time `json:"recording_end"`
		ID             string    `json:"id"`
		MeetingID      string    `json:"meeting_id"`
		FileType       string    `json:"file_type"`
		FilePath       string    `json:"file_path"`
		Status         string    `json:"status"`
		FileSize       int       `json:"file_size"`
	} `json:"recording_files"`

	Duration       int `json:"duration"`
	TotalSize      int `json:"total_size"`
	RecordingCount int `json:"recording_count"`
	MeetingNumber  int `json:"meeting_number"`
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
