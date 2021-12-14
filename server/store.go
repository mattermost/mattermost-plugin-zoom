// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	postMeetingKey        = "post_meeting_"
	zoomStateKeyPrefix    = "zoomuserstate"
	zoomUserByMMID        = "zoomtoken_"
	zoomUserByZoomID      = "zoomtokenbyzoomid_"
	zoomSuperUserTokenKey = "zoomSuperUserToken_"

	meetingPostIDTTL  = 60 * 60 * 24 // One day
	oAuthUserStateTTL = 60 * 5       // 5 minutes
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
	// according to the definition encoded would be nil
	encoded, err := p.API.KVGet(zoomUserByMMID + userID)

	if err != nil {
		return errors.Wrap(err, "could not find OAuth user info")
	}
	if encoded == nil {
		return errors.New("you are not connected to Zoom yet")
	}

	var info zoom.OAuthUserInfo
	if err := json.Unmarshal(encoded, &info); err != nil {
		return errors.Wrap(err, "could not decode OAuth user info")
	}

	appErr := p.API.KVDelete(zoomUserByMMID + userID)
	if appErr != nil {
		return appErr
	}

	appErr = p.API.KVDelete(zoomUserByZoomID + info.ZoomID)
	if appErr != nil {
		return appErr
	}

	return nil
}

// storeOAuthUserState generates an OAuth user state that contains the user ID & channel ID,
// then stores it in the KV store with and expiry of 5 minutes.
func (p *Plugin) storeOAuthUserState(userID string, channelID string, justConnect bool) *model.AppError {
	key := getOAuthUserStateKey(userID)
	connectString := falseString
	if justConnect {
		connectString = trueString
	}
	state := fmt.Sprintf("%s_%s_%s_%s", model.NewId()[0:15], userID, channelID, connectString)
	return p.API.KVSetWithExpiry(key, []byte(state), oAuthUserStateTTL)
}

// fetchOAuthUserState retrieves the OAuth user state from the KV store by the user ID.
func (p *Plugin) fetchOAuthUserState(userID string) (string, *model.AppError) {
	key := getOAuthUserStateKey(userID)
	state, err := p.API.KVGet(key)
	if err != nil {
		return "", err
	}

	return string(state), nil
}

// deleteUserState deletes the stored the OAuth user state from the KV store for the given userID.
func (p *Plugin) deleteUserState(userID string) *model.AppError {
	key := getOAuthUserStateKey(userID)
	return p.API.KVDelete(key)
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

// getOAuthUserStateKey generates and returns the key for storing the OAuth user state in the KV store.
func getOAuthUserStateKey(userID string) string {
	return fmt.Sprintf("%s_%s", zoomStateKeyPrefix, userID)
}

func (p *Plugin) getSuperuserToken() (*oauth2.Token, error) {
	var token oauth2.Token
	rawToken, appErr := p.API.KVGet(zoomSuperUserTokenKey)
	if appErr != nil {
		return nil, appErr
	}
	if len(rawToken) == 0 {
		return nil, nil
	}

	err := json.Unmarshal(rawToken, &token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func (p *Plugin) setSuperUserToken(token *oauth2.Token) error {
	rawToken, err := json.Marshal(token)
	if err != nil {
		return err
	}

	appErr := p.API.KVSet(zoomSuperUserTokenKey, rawToken)
	if appErr != nil {
		return appErr
	}

	return nil
}

func (p *Plugin) removeSuperUserToken() error {
	appErr := p.API.KVDelete(zoomSuperUserTokenKey)
	if appErr != nil {
		return appErr
	}

	return nil
}
