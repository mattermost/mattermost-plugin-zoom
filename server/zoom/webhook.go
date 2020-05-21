// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"time"
)

const (
	WebhookStatusStarted         = "STARTED"
	WebhookStatusEnded           = "ENDED"
	RecordingWebhookTypeComplete = "RECORDING_MEETING_COMPLETED"
	RecentlyCreated              = "RECENTLY_CREATED"
)

type Webhook struct {
	ID     int    `schema:"id"`
	UUID   string `schema:"uuid"`
	Status string `schema:"status"`
	HostID string `schema:"host_id"`
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
