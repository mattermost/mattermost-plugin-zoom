// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"encoding/json"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
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

// Client interface for Zoom
type Client interface {
	GetMeeting(meetingID int) (*Meeting, error)
	GetUser(user *model.User, firstConnect bool) (*User, *AuthError)
	CreateMeeting(user *User, topic string) (*Meeting, error)
}

type PluginAPI interface {
	GetZoomSuperUserToken() (*oauth2.Token, error)
	SetZoomSuperUserToken(*oauth2.Token) error
	GetZoomOAuthUserInfo(userID string) (*OAuthUserInfo, error)
	UpdateZoomOAuthUserInfo(userID string, info *OAuthUserInfo) error
}
