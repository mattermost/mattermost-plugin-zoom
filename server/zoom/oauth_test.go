// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package zoom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestCreateMeetingAddressesTheTokenOwner(t *testing.T) {
	for name, tc := range map[string]struct {
		isAccountLevel bool
		expectedPath   string
	}{
		"user-level token posts to /users/me": {
			isAccountLevel: false,
			expectedPath:   "/users/me/meetings",
		},
		"account-level token posts to the user's email": {
			isAccountLevel: true,
			expectedPath:   "/users/someone@example.com/meetings",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id": 123, "join_url": "https://zoom.us/j/123"}`))
			}))
			defer server.Close()

			client := NewOAuthClient(&oauth2.Token{AccessToken: "token"}, &oauth2.Config{}, "", server.URL, tc.isAccountLevel, nil)

			meeting, err := client.CreateMeeting(&User{Email: "someone@example.com"}, "Topic")
			require.NoError(t, err)
			assert.Equal(t, 123, meeting.ID)
			assert.Equal(t, tc.expectedPath, gotPath)
		})
	}
}

func TestCreateMeetingRequestOmitsUnsetOptionalFields(t *testing.T) {
	b, err := json.Marshal(CreateMeetingRequest{Topic: "Topic", Type: MeetingTypeInstant})
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(b, &body))
	assert.NotContains(t, body, "tracking_fields")

	settings, ok := body["settings"].(map[string]any)
	require.True(t, ok, "settings should be present")
	for _, field := range []string{"registration_type", "audio", "auto_recording", "global_dial_in_countries"} {
		assert.NotContains(t, settings, field)
	}
}
