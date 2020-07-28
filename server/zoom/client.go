// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

// Client defines a common interface for the API and OAuth Zoom clients
type Client interface {
	CompleteCompliance(payload DeauthorizationPayload) error
	GetMeeting(meetingID int) (*Meeting, error)
	GetUser(userID string) (*User, error)
}
