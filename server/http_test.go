// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
)

func TestHandleChannelPreferenceAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		headerUserID         string
		payloadUserID        string
		expectStatus         int
		expectKVUpdate       bool
		expectPermissionCall bool
	}{
		"success": {
			headerUserID:         "admin",
			payloadUserID:        "admin",
			expectStatus:         http.StatusOK,
			expectKVUpdate:       true,
			expectPermissionCall: true,
		},
		"missing header": {
			headerUserID:  "",
			payloadUserID: "admin",
			expectStatus:  http.StatusUnauthorized,
		},
		"mismatched user": {
			headerUserID:  "user",
			payloadUserID: "admin",
			expectStatus:  http.StatusUnauthorized,
		},
	} {
		t.Run(name, func(t *testing.T) {
			api := &plugintest.API{}
			p := Plugin{}
			p.SetAPI(api)

			api.On("LogError", mock.Anything).Maybe().Return()
			api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
			api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
			api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

			if tc.expectPermissionCall {
				api.On("HasPermissionTo", tc.payloadUserID, model.PermissionManageSystem).Return(true)
			}

			if tc.expectKVUpdate {
				api.On("KVGet", zoomChannelSettings).Return([]byte{}, nil)
				api.On("KVSet", zoomChannelSettings, mock.AnythingOfType("[]uint8")).Return(nil)
			}

			reqPayload := &model.SubmitDialogRequest{
				UserId:    tc.payloadUserID,
				ChannelId: "channel",
				Submission: map[string]interface{}{
					"preference": "allow",
				},
			}
			body, err := json.Marshal(reqPayload)
			require.NoError(t, err)

			request := httptest.NewRequest(http.MethodPost, pathChannelPreference, bytes.NewReader(body))
			if tc.headerUserID != "" {
				request.Header.Set(MattermostUserIDHeader, tc.headerUserID)
			}

			rr := httptest.NewRecorder()

			p.handleChannelPreference(rr, request)

			assert.Equal(t, tc.expectStatus, rr.Code)
			api.AssertExpectations(t)
		})
	}
}
