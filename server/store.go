// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	postMeetingKey     = "post_meeting_"
	zoomStateKeyPrefix = "zoomuserstate"
	zoomUserByMMID     = "zoomtoken_"
	zoomUserByZoomID   = "zoomtokenbyzoomid_"
)

func (p *Plugin) storeOAuthUserInfo(info *zoom.OAuthUserInfo) error {
	config := p.getConfiguration()

	encryptedToken, err := encrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		return errors.Wrap(err, "could not encrypt OAuth token")
	}
	info.OAuthToken.AccessToken = encryptedToken

	encoded, err := json.Marshal(info)
	if err != nil {
		return err
	}

	if err := p.API.KVSet(zoomUserByMMID+info.UserID, encoded); err != nil {
		return err
	}

	if err := p.API.KVSet(zoomUserByZoomID+info.ZoomID, encoded); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) fetchOAuthUserInfo(tokenKey, userID string) (*zoom.OAuthUserInfo, error) {
	encoded, appErr := p.API.KVGet(tokenKey + userID)
	if appErr != nil || encoded == nil {
		return nil, errors.New("must connect user account to Zoom first")
	}

	var info zoom.OAuthUserInfo
	if err := json.Unmarshal(encoded, &info); err != nil {
		return nil, errors.New("could not to parse OAauth access token")
	}

	return &info, nil
}

func (p *Plugin) disconnectOAuthUser(userID string) error {
	encoded, err := p.API.KVGet(zoomUserByMMID + userID)
	if err != nil {
		return errors.Wrap(err, "could not find OAuth user info")
	}

	var info zoom.OAuthUserInfo
	if err := json.Unmarshal(encoded, &info); err != nil {
		return errors.Wrap(err, "could not decode OAuth user info")
	}

	errByMattermostID := p.API.KVDelete(zoomUserByMMID + userID)
	errByZoomID := p.API.KVDelete(zoomUserByZoomID + info.ZoomID)
	if errByMattermostID != nil {
		return errByMattermostID
	}
	if errByZoomID != nil {
		return errByZoomID
	}
	return nil
}

// storeUserState stores the user state with the corresponding channelID
func (p *Plugin) storeUserState(userID string, channelID string) error {
	state := fmt.Sprintf("%s_%s", zoomStateKeyPrefix, userID)
	return p.API.KVSet(state, []byte(channelID))
}

// deleteUserState deletes the user state from the store and returns channelID from the deleted state
func (p *Plugin) deleteUserState(state string) (string, error) {
	channelID, err := p.API.KVGet(state)
	if err != nil {
		return "", errors.Wrap(err, "missing stored state")
	}

	if err = p.API.KVDelete(state); err != nil {
		return "", errors.Wrap(err, "failed to delete state from db")
	}

	return string(channelID), nil
}

func (p *Plugin) storeMeetingPostID(meetingID int, postID string) *model.AppError {
	key := fmt.Sprintf("%v%v", postMeetingKey, meetingID)
	bytes := []byte(postID)
	return p.API.KVSetWithExpiry(key, bytes, meetingPostIDTTL)
}

func (p *Plugin) fetchMeetingPostID(meetingID string) (string, *model.AppError) {
	key := fmt.Sprintf("%v%v", postMeetingKey, meetingID)
	postID, appErr := p.API.KVGet(key)
	if appErr != nil {
		p.API.LogDebug("Could not get meeting post from KVStore", "err", appErr.Error())
		return "", appErr
	}

	if postID == nil {
		p.API.LogWarn("Stored meeting not found")
		return "", appErr
	}

	return string(postID), nil
}

func (p *Plugin) deleteMeetingPostID(postID string) *model.AppError {
	key := fmt.Sprintf("%v%v", postMeetingKey, postID)
	return p.API.KVDelete(key)
}
