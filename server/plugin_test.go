// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest/mock"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

func TestPlugin(t *testing.T) {
	// Mock zoom server
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
		HasPermissionToChannel bool
	}{
		"UnauthorizedMeetingRequest": {
			Request:                noAuthMeetingRequest,
			ExpectedStatusCode:     http.StatusUnauthorized,
			HasPermissionToChannel: true,
		},
		"ValidMeetingRequest": {
			Request:                meetingRequest,
			ExpectedStatusCode:     http.StatusOK,
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

			api.On("HasPermissionToChannel", "theuserid", "thechannelid", model.PermissionCreatePost).Return(tc.HasPermissionToChannel)

			api.On("GetChannelMember", "thechannelid", "theuserid").Return(&model.ChannelMember{}, nil)

			api.On("GetPost", "thepostid").Return(&model.Post{Props: map[string]interface{}{}}, nil)
			api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)
			api.On("UpdatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)
			api.On("GetPostsSince", "thechannelid", mock.AnythingOfType("int64")).Return(&model.PostList{}, nil)

			api.On("KVSetWithExpiry", fmt.Sprintf("%v%v", postMeetingKey, 234), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("int64")).Return(nil)
			api.On("KVSetWithExpiry", fmt.Sprintf("%v%v", postMeetingKey, 123), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("int64")).Return(nil)
			api.On("KVSetWithExpiry", "zoomuserstate_theuserid", mock.AnythingOfType("[]uint8"), mock.AnythingOfType("int64")).Return(nil)

			api.On("KVGet", fmt.Sprintf("%v%v", postMeetingKey, 234)).Return([]byte("thepostid"), nil)
			api.On("KVGet", fmt.Sprintf("%v%v", postMeetingKey, 123)).Return([]byte("thepostid"), nil)

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
				ZoomAPIURL:    ts.URL,
				WebhookSecret: "thewebhooksecret",
				EncryptionKey: "4Su-mLR7N6VwC6aXjYhQoT0shtS9fKz+",
			})
			p.SetAPI(api)
			p.tracker = telemetry.NewTracker(nil, "", "", "", "", "", false)

			err = p.OnActivate()
			require.Nil(t, err)

			tc.Request.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, w, tc.Request)
			assert.Equal(t, tc.ExpectedStatusCode, w.Result().StatusCode)
		})
	}
}
