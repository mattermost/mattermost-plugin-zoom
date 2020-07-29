// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
)

const postMeetingKey = "post_meeting_"

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

	if err := p.API.KVSet(zoomTokenKey+info.UserID, encoded); err != nil {
		return err
	}

	if err := p.API.KVSet(zoomTokenKeyByZoomID+info.ZoomID, encoded); err != nil {
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
	encoded, err := p.API.KVGet(zoomTokenKey + userID)
	if err != nil {
		return errors.Wrap(err, "could not find OAuth user info")
	}

	var info zoom.OAuthUserInfo
	if err := json.Unmarshal(encoded, &info); err != nil {
		return errors.Wrap(err, "could not decode OAuth user info")
	}

	errByMattermostID := p.API.KVDelete(zoomTokenKey + userID)
	errByZoomID := p.API.KVDelete(zoomTokenKeyByZoomID + info.ZoomID)
	if errByMattermostID != nil {
		return errByMattermostID
	}
	if errByZoomID != nil {
		return errByZoomID
	}
	return nil
}

func (p *Plugin) storeUserState(userID, channelID string) (string, error) {
	key := fmt.Sprintf("%v_%v", model.NewId()[0:15], userID)
	state := fmt.Sprintf("%v_%v", key, channelID)
	if err := p.API.KVSet(key, []byte(state)); err != nil {
		return "", errors.Wrap(err, "could not store user state")
	}

	return state, nil
}

func (p *Plugin) deleteUserState(state string) error {
	stateComponents := strings.Split(state, "_")
	key := fmt.Sprintf("%v_%v", stateComponents[0], stateComponents[1])
	storedState, err := p.API.KVGet(key)
	if err != nil {
		return errors.Wrap(err, "missing stored state")
	}

	if string(storedState) != state {
		return errors.Wrap(err, "invalid state")
	}

	if err = p.API.KVDelete(state); err != nil {
		return errors.Wrap(err, "failed to delete state from db")
	}

	return nil
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
