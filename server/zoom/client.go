// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"encoding/json"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
)

// AuthError represents a Zoom authentication error
type AuthError struct {
	Message string `json:"message"`
	Err     error  `json:"err"`
}

func (err *AuthError) Error() string {
	msg, _ := json.Marshal(err)
	return string(msg)
}

var errNotFound = errors.New("not found")

type Client interface {
	GetMeeting(meetingID int) (*Meeting, error)
	GetUser(user *model.User) (*User, *AuthError)
}
