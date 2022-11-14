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

	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest/mock"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

func TestPlugin(t *testing.T) {
	t.Skip("need to fix this test and use the new plugin-api lib")
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
		"UnauthorizedChannelPermissions": {
			Request:                unauthorizedUserRequest,
			ExpectedStatusCode:     http.StatusInternalServerError,
			HasPermissionToChannel: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			botUserID := "yei0BahL3cohya8vuaboShaeSi"

			api := &plugintest.API{}

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
			api.On("GetPreferencesForUser", mock.AnythingOfType("string")).Return([]model.Preference{
				{
					UserId:   "test-userid",
					Category: zoomPreferenceCategory,
					Name:     zoomPMISettingName,
					Value:    trueString,
				},
			}, nil)

			p := Plugin{}
			p.setConfiguration(&configuration{
				ZoomAPIURL:    ts.URL,
				APIKey:        "theapikey",
				APISecret:     "theapisecret",
				WebhookSecret: "thewebhooksecret",
			})
			p.SetAPI(api)
			p.tracker = telemetry.NewTracker(nil, "", "", "", "", "", false)

			// TODO: fixme
			// helpers := &plugintest.Helpers{}
			// helpers.On("EnsureBot", mock.AnythingOfType("*model.Bot")).Return(botUserID, nil)
			// p.SetHelpers(helpers)

			err = p.OnActivate()
			require.Nil(t, err)

			tc.Request.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, w, tc.Request)
			assert.Equal(t, tc.ExpectedStatusCode, w.Result().StatusCode)
		})
	}
}
