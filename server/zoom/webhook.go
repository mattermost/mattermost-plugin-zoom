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

	EventTypeMeetingStarted    EventType = "meeting.started"
	EventTypeMeetingEnded      EventType = "meeting.ended"
	EventTypeParticipantJoined EventType = "meeting.participant_joined"
	EventTypeParticipantLeft   EventType = "meeting.participant_left"
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

type Webhook struct {
	Event   EventType   `json:"event"`
	Payload interface{} `json:"payload"`
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

type ParticipantJoinedLeftEvent struct {
	EventType      EventType                         `json:"event"`
	EventTimestamp int                               `json:"event_ts"`
	Payload        ParticipantJoinedLeftEventPayload `json:"payload"`
}

type ParticipantJoinedLeftEventPayload struct {
	AccountID string                           `json:"account_id"`
	Object    ParticipantJoinedLeftEventObject `json:"object"`
}

type MeetingType int

const (
	Prescheduled            MeetingType = 0
	Instant                             = 1
	Scheduled                           = 2
	RecurringNoFixedTime                = 3
	PersonalMeetingRoom                 = 4
	PersonalAudioConference             = 7
	RecurringFixedTime                  = 8
)

type ParticipantJoinedLeftEventObject struct {
	ID          string      `json:"id"`
	UUID        string      `json:"uuid"`
	HostID      string      `json:"host_id"`
	Topic       string      `json:"topic"`
	Type        MeetingType `json:"type"`
	StartTime   string      `json:"start_time"`
	TimeZone    string      `json:"timezone"`
	Duration    int         `json:"duration"`
	Participant Participant `json:"participant"`
}

type Participant struct {
	UserID            string `json:"user_id"`
	UserName          string `json:"user_name"`
	ID                string `json:"id"`
	LeaveTime         string `json:"leave_time"`
	LeaveReason       string `json:"leave_reason"`
	Email             string `json:"email"`
	RegistrantID      string `json:"registrant_id"`
	ParticipantUserID string `json:"participant_user_id"`
}
