// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"encoding/json"

	"github.com/mattermost/mattermost-server/v5/model"
)

// Client defines a common interface for the API and OAuth Zoom clients
type Client interface {
	GetMeeting(meetingID int) (*Meeting, error)
	GetUser(user *model.User) (*User, *AuthError)
}

// AuthError represents a Zoom authentication error
type AuthError struct {
	Message string `json:"message"`
	Err     error  `json:"err"`
}

func (err *AuthError) Error() string {
	msg, _ := json.Marshal(err)
	return string(msg)
}
