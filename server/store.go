// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
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
	zoomChannelSettings   = "zoomChannelSettings"
	zoomUserPreferenceKey = "zoomUserPreference_%s"

	meetingPostIDTTL  = 60 * 60 * 24 // One day
	oAuthUserStateTTL = 60 * 5       // 5 minutes
)

type ZoomChannelSettingsMapValue struct {
	Preference string
}

type ZoomChannelSettingsMap map[string]ZoomChannelSettingsMapValue

func (p *Plugin) storeOAuthUserInfo(info *zoom.OAuthUserInfo) error {
	config := p.getConfiguration()

	encryptedToken, err := encrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		return errors.Wrap(err, "could not encrypt OAuth token")
	}

	original := info.OAuthToken.AccessToken
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

	info.OAuthToken.AccessToken = original
	return nil
}

func (p *Plugin) fetchOAuthUserInfo(tokenKey, userID string) (*zoom.OAuthUserInfo, error) {
	config := p.getConfiguration()

	encoded, appErr := p.API.KVGet(tokenKey + userID)
	if appErr != nil || encoded == nil {
		return nil, errors.New("must connect user account to Zoom first")
	}

	var info zoom.OAuthUserInfo
	if err := json.Unmarshal(encoded, &info); err != nil {
		return nil, errors.New("could not parse OAuth access token")
	}

	plainToken, err := decrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		return nil, errors.New("could not decrypt OAuth access token")
	}

	info.OAuthToken.AccessToken = plainToken

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
	b := []byte(postID)
	return p.API.KVSetWithExpiry(key, b, meetingPostIDTTL)
}

func (p *Plugin) fetchMeetingPostID(meetingID string) (string, error) {
	key := fmt.Sprintf("%v%v", postMeetingKey, meetingID)
	var postID string
	if err := p.client.KV.Get(key, &postID); err != nil {
		p.client.Log.Debug("Could not get meeting post from KVStore", "error", err.Error())
		return "", err
	}

	if postID == "" {
		return "", errors.New("stored meeting post ID not found")
	}

	return postID, nil
}

func (p *Plugin) deleteMeetingPostID(postID string) error {
	key := fmt.Sprintf("%v%v", postMeetingKey, postID)
	return p.client.KV.Delete(key)
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

func (p *Plugin) storeZoomChannelSettings(channelID string, zoomChannelSettingsMapValue ZoomChannelSettingsMapValue) error {
	b, appErr := p.API.KVGet(zoomChannelSettings)
	if appErr != nil {
		return errors.New(appErr.Message)
	}

	var zoomChannelSettingsMap ZoomChannelSettingsMap
	if len(b) != 0 {
		if err := json.Unmarshal(b, &zoomChannelSettingsMap); err != nil {
			return err
		}
	} else {
		zoomChannelSettingsMap = ZoomChannelSettingsMap{}
	}

	zoomChannelSettingsMap[channelID] = zoomChannelSettingsMapValue
	b, err := json.Marshal(zoomChannelSettingsMap)
	if err != nil {
		return err
	}

	if appErr := p.API.KVSet(zoomChannelSettings, b); appErr != nil {
		return errors.New(appErr.Message)
	}

	return nil
}

func (p *Plugin) listZoomChannelSettings() (ZoomChannelSettingsMap, error) {
	b, appErr := p.API.KVGet(zoomChannelSettings)
	if appErr != nil {
		return nil, errors.New(appErr.Message)
	}

	if len(b) == 0 {
		return ZoomChannelSettingsMap{}, nil
	}

	var zoomChannelSettingsMap ZoomChannelSettingsMap
	if err := json.Unmarshal(b, &zoomChannelSettingsMap); err != nil {
		return nil, err
	}

	return zoomChannelSettingsMap, nil
}

func (p *Plugin) storeUserPreference(userID, value string) error {
	if _, err := p.client.KV.Set(fmt.Sprintf(zoomUserPreferenceKey, userID), &value); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) getUserPreference(userID string) (string, error) {
	var value string
	if err := p.client.KV.Get(fmt.Sprintf(zoomUserPreferenceKey, userID), &value); err != nil {
		return "", err
	}

	if value == "" {
		/*
			If preference is not stored in kv store, check in the preferences table for user preference
		*/
		preferences, reqErr := p.API.GetPreferencesForUser(userID)
		if reqErr != nil {
			return "", errors.New(settingDataError)
		}

		for _, preference := range preferences {
			if preference.UserId == userID && preference.Category == zoomPreferenceCategory && preference.Name == zoomPMISettingName {
				/*
					If found return the value, and remove user preference from preferences table and store in kv store
				*/
				if err := p.storeUserPreference(userID, preference.Value); err != nil {
					p.client.Log.Error("Unable to store user preference", "UserID", userID, "Error", err.Error())
					return preference.Value, nil
				}

				// Delete the preference from preferences table as we have already stored it in kv store
				if err := p.API.DeletePreferencesForUser(userID, []model.Preference{{
					UserId:   userID,
					Category: zoomPreferenceCategory,
					Name:     zoomPMISettingName,
					Value:    preference.Value,
				}}); err != nil {
					p.client.Log.Error("Unable to delete user preference from db", "UserID", userID, "Error", err.Error())
				}

				return preference.Value, nil
			}
		}

		return "", nil
	}

	return value, nil
}
