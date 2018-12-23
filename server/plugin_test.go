// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/mattermost/mattermost-server/plugin/plugintest"
	"github.com/mattermost/mattermost-server/plugin/plugintest/mock"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

func TestPlugin(t *testing.T) {
	// Mock zoom server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/theuseremail" {
			user := &zoom.User{
				ID:  "thezoomuserid",
				Pmi: 123,
			}

			str, _ := json.Marshal(user)

			w.Write([]byte(str))
		} else if r.URL.Path == "/users/theuseremail/meetings/" {
			meeting := &zoom.Meeting{
				ID: 234,
			}

			str, _ := json.Marshal(meeting)

			w.Write([]byte(str))
		}
	}))
	defer ts.Close()

	validMeetingRequest := httptest.NewRequest("POST", "/api/v1/meetings", strings.NewReader("{\"channel_id\": \"thechannelid\"}"))
	validMeetingRequest.Header.Add("Mattermost-User-Id", "theuserid")

	noAuthMeetingRequest := httptest.NewRequest("POST", "/api/v1/meetings", strings.NewReader("{\"channel_id\": \"thechannelid\"}"))

	personalMeetingRequest := httptest.NewRequest("POST", "/api/v1/meetings", strings.NewReader("{\"channel_id\": \"thechannelid\", \"personal\": true}"))
	personalMeetingRequest.Header.Add("Mattermost-User-Id", "theuserid")

	validWebhookRequest := httptest.NewRequest("POST", "/webhook?secret=thewebhooksecret", strings.NewReader("id=234&uuid=1dnv2x3XRiMdoVIwzms5lA%3D%3D&status=ENDED&host_id=iQZt4-f1ZQp2tgWwx-p1mQ"))
	validWebhookRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	validStartedWebhookRequest := httptest.NewRequest("POST", "/webhook?secret=thewebhooksecret", strings.NewReader("id=234&uuid=1dnv2x3XRiMdoVIwzms5lA%3D%3D&status=STARTED&host_id=iQZt4-f1ZQp2tgWwx-p1mQ"))

	noSecretWebhookRequest := httptest.NewRequest("POST", "/webhook", strings.NewReader("id=234&uuid=1dnv2x3XRiMdoVIwzms5lA%3D%3D&status=ENDED&host_id=iQZt4-f1ZQp2tgWwx-p1mQ"))

	for name, tc := range map[string]struct {
		Request            *http.Request
		CreatePostError    *model.AppError
		ExpectedStatusCode int
	}{
		"UnauthorizedMeetingRequest": {
			Request:            noAuthMeetingRequest,
			ExpectedStatusCode: http.StatusUnauthorized,
		},
		"ValidMeetingRequest": {
			Request:            validMeetingRequest,
			ExpectedStatusCode: http.StatusOK,
		},
		"ValidPersonalMeetingRequest": {
			Request:            personalMeetingRequest,
			ExpectedStatusCode: http.StatusOK,
		},
		"ValidWebhookRequest": {
			Request:            validWebhookRequest,
			ExpectedStatusCode: http.StatusOK,
		},
		"ValidStartedWebhookRequest": {
			Request:            validStartedWebhookRequest,
			ExpectedStatusCode: http.StatusOK,
		},
		"NoSecretWebhookRequest": {
			Request:            noSecretWebhookRequest,
			ExpectedStatusCode: http.StatusUnauthorized,
		},
	} {
		t.Run(name, func(t *testing.T) {
			api := &plugintest.API{}

			api.On("GetUser", "theuserid").Return(&model.User{
				Id:    "theuserid",
				Email: "theuseremail",
			}, (*model.AppError)(nil))

			api.On("GetChannelMember", "thechannelid", "theuserid").Return(&model.ChannelMember{}, (*model.AppError)(nil))

			api.On("GetPost", "thepostid").Return(&model.Post{Props: map[string]interface{}{}}, (*model.AppError)(nil))
			api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, (*model.AppError)(nil))
			api.On("UpdatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, (*model.AppError)(nil))

			api.On("KVSet", fmt.Sprintf("%v%v", POST_MEETING_KEY, 234), mock.AnythingOfType("[]uint8")).Return((*model.AppError)(nil))
			api.On("KVSet", fmt.Sprintf("%v%v", POST_MEETING_KEY, 123), mock.AnythingOfType("[]uint8")).Return((*model.AppError)(nil))

			api.On("KVGet", fmt.Sprintf("%v%v", POST_MEETING_KEY, 234)).Return([]byte("thepostid"), (*model.AppError)(nil))
			api.On("KVGet", fmt.Sprintf("%v%v", POST_MEETING_KEY, 123)).Return([]byte("thepostid"), (*model.AppError)(nil))

			api.On("KVDelete", fmt.Sprintf("%v%v", POST_MEETING_KEY, 234)).Return((*model.AppError)(nil))

			p := Plugin{}
			p.setConfiguration(&configuration{
				ZoomAPIURL:    ts.URL,
				APIKey:        "theapikey",
				APISecret:     "theapisecret",
				WebhookSecret: "thewebhooksecret",
			})
			p.SetAPI(api)
			err := p.OnActivate()
			assert.Nil(t, err)

			w := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, w, tc.Request)
			assert.Equal(t, tc.ExpectedStatusCode, w.Result().StatusCode)
		})
	}
}
