// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	postMeetingKey        = "post_meeting_"
	meetingChannelKey     = "meeting_channel_"
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

// meetingPostKey returns a KV-safe key for a given Zoom meeting UUID.
// Zoom UUIDs can contain '/' and '=' which may cause issues in KV keys.
func meetingPostKey(meetingUUID string) string {
	return postMeetingKey + url.PathEscape(meetingUUID)
}

func (p *Plugin) storeMeetingPostID(meetingUUID string, postID string) *model.AppError {
	key := meetingPostKey(meetingUUID)
	b := []byte(postID)
	return p.API.KVSetWithExpiry(key, b, meetingPostIDTTL)
}

func (p *Plugin) fetchMeetingPostID(meetingUUID string) (string, error) {
	key := meetingPostKey(meetingUUID)
	var postIDData []byte
	if err := p.client.KV.Get(key, &postIDData); err != nil {
		p.client.Log.Debug("Could not get meeting post from KVStore", "error", err.Error())
		return "", err
	}

	if postIDData == nil {
		return "", errors.New("stored meeting post ID not found")
	}

	return string(postIDData), nil
}

// meetingChannelEntry stores metadata about a meeting-to-channel mapping.
type meetingChannelEntry struct {
	ChannelID      string `json:"channel_id"`
	IsSubscription bool   `json:"is_subscription"`
	CreatedBy      string `json:"created_by"`
}

// Ad-hoc meeting channel entries expire after 24 hours. This must be long
// enough to cover the full meeting duration (which can be many hours) plus
// the post-meeting window for recording/transcript webhooks to arrive.
const adHocMeetingChannelTTL = 60 * 60 * 24

func meetingChannelKVKey(meetingID int) string {
	return fmt.Sprintf("%v%v", meetingChannelKey, meetingID)
}

func (p *Plugin) storeSubscriptionForMeeting(meetingID int, channelID, userID string) error {
	existing, appErr := p.getMeetingChannelEntry(meetingID)
	if appErr != nil {
		return appErr
	}
	if existing != nil && existing.IsSubscription {
		if existing.ChannelID == channelID && existing.CreatedBy == userID {
			return nil
		}
		return errors.New("meeting already has an existing subscription")
	}

	entry := meetingChannelEntry{
		ChannelID:      channelID,
		IsSubscription: true,
		CreatedBy:      userID,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if appErr := p.API.KVSet(meetingChannelKVKey(meetingID), data); appErr != nil {
		return appErr
	}
	return nil
}

func (p *Plugin) storeChannelForMeeting(meetingID int, channelID string) error {
	key := meetingChannelKVKey(meetingID)

	existing, appErr := p.getMeetingChannelEntry(meetingID)
	if appErr != nil {
		return appErr
	}
	if existing != nil && existing.IsSubscription {
		return nil
	}

	entry := meetingChannelEntry{
		ChannelID:      channelID,
		IsSubscription: false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if appErr := p.API.KVSetWithExpiry(key, data, adHocMeetingChannelTTL); appErr != nil {
		return appErr
	}
	return nil
}

func (p *Plugin) getMeetingChannelEntry(meetingID int) (*meetingChannelEntry, *model.AppError) {
	key := meetingChannelKVKey(meetingID)
	raw, appErr := p.API.KVGet(key)
	if appErr != nil {
		return nil, appErr
	}
	if raw == nil {
		return nil, nil
	}

	var entry meetingChannelEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		p.API.LogWarn("failed to unmarshal meeting channel entry",
			"key", key,
			"error", err.Error(),
		)
		return nil, nil
	}
	if entry.ChannelID == "" {
		return nil, nil
	}
	return &entry, nil
}

func (p *Plugin) fetchChannelForMeeting(meetingID int) (string, *model.AppError) {
	entry, appErr := p.getMeetingChannelEntry(meetingID)
	if appErr != nil {
		return "", appErr
	}
	if entry == nil {
		return "", nil
	}
	return entry.ChannelID, nil
}

func (p *Plugin) deleteChannelForMeeting(meetingID int) error {
	key := meetingChannelKVKey(meetingID)
	return p.client.KV.Delete(key)
}

const kvListPerPage = 100

func (p *Plugin) listAllMeetingSubscriptions(userID string) (map[string]string, error) {
	subscriptions := make(map[string]string)

	for page := 0; ; page++ {
		keys, appErr := p.API.KVList(page, kvListPerPage)
		if appErr != nil {
			return nil, errors.New(appErr.Message)
		}

		for _, key := range keys {
			if !strings.HasPrefix(key, meetingChannelKey) {
				continue
			}

			raw, kvErr := p.API.KVGet(key)
			if kvErr != nil || raw == nil {
				continue
			}

			var entry meetingChannelEntry
			if err := json.Unmarshal(raw, &entry); err != nil || entry.ChannelID == "" {
				continue
			}

			if !entry.IsSubscription {
				continue
			}

			if entry.CreatedBy != userID {
				continue
			}

			meetingID := strings.TrimPrefix(key, meetingChannelKey)
			subscriptions[meetingID] = entry.ChannelID
		}

		if len(keys) < kvListPerPage {
			break
		}
	}

	return subscriptions, nil
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
