// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/telemetry"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

func TestPlugin(t *testing.T) {
	// Mock Zoom server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/theuseremail" {
			user := &zoom.User{
				ID:    "thezoomuserid",
				Email: "theuseremail",
				Pmi:   123,
			}

			str, _ := json.Marshal(user)

			if _, err := w.Write(str); err != nil {
				require.NoError(t, err)
			}
		}
		if r.URL.Path == "/users/theuseremail/meetings" {
			meeting := &zoom.Meeting{
				ID: 234,
			}

			str, _ := json.Marshal(meeting)

			if _, err := w.Write(str); err != nil {
				require.NoError(t, err)
			}
		}
	}))

	defer ts.Close()

	noAuthMeetingRequest := httptest.NewRequest("POST", "/api/v1/meetings", strings.NewReader("{\"channel_id\": \"thechannelid\"}"))

	restrictMeetingRequest := httptest.NewRequest("POST", "/api/v1/meetings", strings.NewReader("{\"channel_id\": \"thechannelid\"}"))
	restrictMeetingRequest.Header.Add("Mattermost-User-Id", "theuserid")

	meetingRequest := httptest.NewRequest("POST", "/api/v1/meetings", strings.NewReader("{\"channel_id\": \"thechannelid\"}"))
	meetingRequest.Header.Add("Mattermost-User-Id", "theuserid")

	endedPayload := `{"event": "meeting.ended", "payload": {"object": {"id": "234"}}}`
	validStoppedWebhookRequest := httptest.NewRequest("POST", "/webhook?secret=thewebhooksecret", strings.NewReader(endedPayload))

	validStartedWebhookRequest := httptest.NewRequest("POST", "/webhook?secret=thewebhooksecret", strings.NewReader(`{"event": "meeting.started"}`))

	noSecretWebhookRequest := httptest.NewRequest("POST", "/webhook", strings.NewReader(endedPayload))

	unauthorizedUserRequest := httptest.NewRequest("POST", "/api/v1/meetings", strings.NewReader("{\"channel_id\": \"thechannelid\", \"personal\": true}"))
	unauthorizedUserRequest.Header.Add("Mattermost-User-Id", "theuserid")

	for name, tc := range map[string]struct {
		Request                *http.Request
		ExpectedStatusCode     int
		MeetingAllowed         bool
		HasPermissionToChannel bool
	}{
		"UnauthorizedMeetingRequest": {
			Request:                noAuthMeetingRequest,
			ExpectedStatusCode:     http.StatusUnauthorized,
			HasPermissionToChannel: true,
		},
		"RestrictMeetingRequest": {
			Request:                restrictMeetingRequest,
			ExpectedStatusCode:     http.StatusOK,
			MeetingAllowed:         false,
			HasPermissionToChannel: true,
		},
		"ValidMeetingRequest": {
			Request:                meetingRequest,
			ExpectedStatusCode:     http.StatusOK,
			MeetingAllowed:         true,
			HasPermissionToChannel: true,
		},
		"ValidStoppedWebhookRequest": {
			Request:                validStoppedWebhookRequest,
			ExpectedStatusCode:     http.StatusOK,
			HasPermissionToChannel: true,
		},
		"ValidStartedWebhookRequest": {
			Request:                validStartedWebhookRequest,
			ExpectedStatusCode:     http.StatusOK,
			HasPermissionToChannel: true,
		},
		"NoSecretWebhookRequest": {
			Request:                noSecretWebhookRequest,
			ExpectedStatusCode:     http.StatusUnauthorized,
			HasPermissionToChannel: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			botUserID := "yei0BahL3cohya8vuaboShaeSi"

			api := &plugintest.API{}

			userInfo, err := json.Marshal(zoom.OAuthUserInfo{
				OAuthToken: &oauth2.Token{
					AccessToken: "2a41c3138d2187a756c51428f78d192e9b88dcf44dd62d1b081ace4ec2241e0a",
				},
			})

			require.Nil(t, err)
			api.On("GetLicense").Return(nil)
			api.On("GetServerVersion").Return("6.2.0")

			api.On("KVGet", "mmi_botid").Return([]byte(botUserID), nil)
			api.On("KVGet", "zoomtoken_theuserid").Return(userInfo, nil)

			api.On("SendEphemeralPost", "theuserid", mock.AnythingOfType("*model.Post")).Return(nil)

			api.On("PatchBot", botUserID, mock.AnythingOfType("*model.BotPatch")).Return(nil, nil)

			api.On("GetUser", "theuserid").Return(&model.User{
				Id:    "theuserid",
				Email: "theuseremail",
			}, nil)

			api.On("GetChannel", "thechannelid").Return(&model.Channel{
				Id:          "thechannelid",
				Type:        model.ChannelTypeOpen,
				DisplayName: "mockChannel",
			}, nil)

			api.On("HasPermissionToChannel", "theuserid", "thechannelid", model.PermissionCreatePost).Return(tc.HasPermissionToChannel)

			api.On("GetChannelMember", "thechannelid", "theuserid").Return(&model.ChannelMember{}, nil)

			api.On("GetPost", "thepostid").Return(&model.Post{Props: map[string]interface{}{}}, nil)
			api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)
			api.On("GetPostsSince", "thechannelid", mock.AnythingOfType("int64")).Return(&model.PostList{}, nil)

			api.On("KVSetWithExpiry", fmt.Sprintf("%v%v", postMeetingKey, 234), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("int64")).Return(nil)
			api.On("KVSetWithExpiry", fmt.Sprintf("%v%v", postMeetingKey, 123), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("int64")).Return(nil)
			api.On("KVSetWithExpiry", "zoomuserstate_theuserid", mock.AnythingOfType("[]uint8"), mock.AnythingOfType("int64")).Return(nil)

			api.On("KVSetWithOptions", "mutex_mmi_bot_ensure", mock.AnythingOfType("[]uint8"), model.PluginKVSetOptions{Atomic: true, OldValue: []uint8(nil), ExpireInSeconds: 15}).Return(true, nil)
			api.On("KVSetWithOptions", "mutex_mmi_bot_ensure", []byte(nil), model.PluginKVSetOptions{ExpireInSeconds: 0}).Return(true, nil)

			api.On("EnsureBotUser", &model.Bot{
				Username:    botUserName,
				DisplayName: botDisplayName,
				Description: botDescription,
			}).Return(botUserID, nil)

			api.On("KVGet", fmt.Sprintf("%v%v", postMeetingKey, 234)).Return([]byte("thepostid"), nil)
			api.On("KVGet", fmt.Sprintf("%v%v", postMeetingKey, 123)).Return([]byte("thepostid"), nil)
			api.On("KVGet", zoomChannelSettings).Return([]byte{}, nil)

			api.On("KVDelete", fmt.Sprintf("%v%v", postMeetingKey, 234)).Return(nil)

			api.On("LogWarn", mock.AnythingOfType("string")).Return()
			api.On("LogDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return()

			path, err := filepath.Abs("..")
			require.Nil(t, err)
			api.On("GetBundlePath").Return(path, nil)
			api.On("SetProfileImage", botUserID, mock.Anything).Return(nil)
			api.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil)

			siteURL := "localhost"
			api.On("GetConfig").Return(&model.Config{
				ServiceSettings: model.ServiceSettings{
					SiteURL: &siteURL,
				},
			})
			p := Plugin{}
			p.setConfiguration(&configuration{
				ZoomAPIURL:              ts.URL,
				WebhookSecret:           "thewebhooksecret",
				EncryptionKey:           "4Su-mLR7N6VwC6aXjYhQoT0shtS9fKz+",
				OAuthClientID:           "clientid",
				OAuthClientSecret:       "clientsecret",
				RestrictMeetingCreation: true,
			})
			p.SetAPI(api)
			p.tracker = telemetry.NewTracker(nil, "", "", "", "", "", telemetry.TrackerConfig{}, nil)

			err = p.OnActivate()
			require.Nil(t, err)

			tc.Request.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, w, tc.Request)
			assert.Equal(t, tc.ExpectedStatusCode, w.Result().StatusCode)
		})
	}
}

func TestHandleChannelPreference(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupAPI           func(*plugintest.API)
		RequestBody        string
		ExpectedStatusCode int
		ExpectedResult     string
	}{
		{
			Name: "HandleChannelPreference: invalid body",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Error decoding dialog request", "Error", mock.AnythingOfType("string")).Once()
			},
			RequestBody: `{
				"user_id":
			}`,
			ExpectedStatusCode: http.StatusBadRequest,
			ExpectedResult:     "invalid character '}' looking for beginning of value\n",
		},
		{
			Name: "HandleChannelPreference: empty user ID",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Invalid user ID", "UserID", "").Once()
			},
			RequestBody: `{
				"user_id": ""
			}`,
			ExpectedStatusCode: http.StatusUnauthorized,
			ExpectedResult:     "Not authorized\n",
		},
		{
			Name: "HandleChannelPreference: insufficient permissions",
			SetupAPI: func(api *plugintest.API) {
				api.On("LogError", "Unable to resolve request due to insufficient permissions", "UserID", "mockUserID").Once()
				api.On("HasPermissionTo", "mockUserID", model.PermissionManageSystem).Return(false).Once()
			},
			RequestBody: `{
				"user_id": "mockUserID"
			}`,
			ExpectedStatusCode: http.StatusForbidden,
			ExpectedResult:     "Insufficient permissions\n",
		},
		{
			Name: "HandleChannelPreference: invalid preference",
			SetupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", "mockUserID", model.PermissionManageSystem).Return(true).Once()
				api.On("LogError", "Invalid request body", "Error", "invalid preference").Once()
			},
			RequestBody: `{
				"user_id": "mockUserID",
				"channel_id": "mockChannelID",
				"submission": {
					  "preference": "Dynamic"
				}
			}`,
			ExpectedStatusCode: http.StatusBadRequest,
			ExpectedResult:     "invalid preference\n",
		},
		{
			Name: "HandleChannelPreference: success",
			SetupAPI: func(api *plugintest.API) {
				api.On("HasPermissionTo", "mockUserID", model.PermissionManageSystem).Return(true).Once()
				api.On("KVGet", zoomChannelSettings).Return([]byte{}, nil).Once()
				api.On("KVSet", zoomChannelSettings, mock.Anything).Return(nil).Once()
			},
			RequestBody: `{
				"user_id": "mockUserID",
				"channel_id": "mockChannelID",
				"submission": {
					  "preference": "restrict"
				}
			}`,
			ExpectedStatusCode: http.StatusOK,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)

			api := &plugintest.API{}
			p := Plugin{}
			p.SetAPI(api)
			p.setConfiguration(&configuration{
				ZoomAPIURL:        "mockURL",
				WebhookSecret:     "thewebhooksecret",
				EncryptionKey:     "4Su-mLR7N6VwC6aXjYhQoT0shtS9fKz+",
				OAuthClientID:     "clientid",
				OAuthClientSecret: "clientsecret",
			})

			test.SetupAPI(api)

			licenseTrue := true
			api.On("GetLicense").Return(&model.License{
				Features: &model.Features{
					Cloud: &licenseTrue,
				},
			})

			defer api.AssertExpectations(t)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, pathChannelPreference, strings.NewReader(test.RequestBody))
			r.Header.Add("Mattermost-User-Id", "mockUserID")
			p.ServeHTTP(&plugin.Context{}, w, r)

			result := w.Result()
			require.NotNil(t, result)
			defer result.Body.Close()

			bodyBytes, err := io.ReadAll(result.Body)
			assert.Nil(err)

			bodyString := string(bodyBytes)
			assert.Equal(bodyString, test.ExpectedResult)
			assert.Equal(result.StatusCode, test.ExpectedStatusCode)
		})
	}
}

func TestIsChannelRestrictedForMeetings(t *testing.T) {
	for _, test := range []struct {
		Name               string
		SetupAPI           func(*plugintest.API)
		ExpectedPreference bool
		ExpectedStatusCode int
		ExpectedError      string
	}{
		{
			Name: "IsChannelRestrictedForMeetings: unable to get channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "mockChannelID").Return(nil, &model.AppError{
					Message: "unable to get channel",
				}).Once()
			},
			ExpectedPreference: false,
			ExpectedError:      "unable to get channel",
		},
		{
			Name: "IsChannelRestrictedForMeetings: unable to get preference",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "mockChannelID").Return(&model.Channel{
					Id:   "mockChannelID",
					Type: model.ChannelTypeOpen,
				}, nil).Once()
				api.On("KVGet", zoomChannelSettings).Return(nil, &model.AppError{
					Message: "unable to get preference",
				}).Once()
			},
			ExpectedPreference: false,
			ExpectedError:      "unable to get preference",
		},
		{
			Name: "IsChannelRestrictedForMeetings: preference not set and channel is public",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "mockChannelID").Return(&model.Channel{
					Id:   "mockChannelID",
					Type: model.ChannelTypeOpen,
				}, nil).Once()
				api.On("KVGet", zoomChannelSettings).Return([]byte{}, nil).Once()
			},
			ExpectedPreference: true,
		},
		{
			Name: "IsChannelRestrictedForMeetings: preference not set and channel is private",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", "mockChannelID").Return(&model.Channel{
					Id:   "mockChannelID",
					Type: model.ChannelTypePrivate,
				}, nil).Once()
				api.On("KVGet", zoomChannelSettings).Return([]byte{}, nil).Once()
			},
			ExpectedPreference: false,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)

			api := &plugintest.API{}
			p := Plugin{}
			p.SetAPI(api)
			p.setConfiguration(&configuration{
				RestrictMeetingCreation: true,
			})

			test.SetupAPI(api)

			preference, err := p.isChannelRestrictedForMeetings("mockChannelID")
			assert.Equal(test.ExpectedPreference, preference)
			if err != nil {
				assert.Equal(test.ExpectedError, err.Error())
			} else {
				assert.Nil(err)
			}
		})
	}
}
