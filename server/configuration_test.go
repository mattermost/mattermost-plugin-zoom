// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValid(t *testing.T) {
	for _, testCase := range []struct {
		description string
		config      *configuration
		errMsg      string
	}{
		{
			description: "valid configuration: pre-registered app",
			config: &configuration{
				EnableOAuth:                 true,
				EncryptionKey:               "abcd",
				UsePreregisteredApplication: true,
				WebhookSecret:               "abcd",
			},
		},
		{
			description: "valid configuration: custom OAuth app",
			config: &configuration{
				EnableOAuth:                 true,
				OAuthClientID:               "client-id",
				OAuthClientSecret:           "client-secret",
				EncryptionKey:               "abcd",
				UsePreregisteredApplication: false,
				WebhookSecret:               "abcd",
			},
		},
		{
			description: "valid configuration: API Keys app",
			config: &configuration{
				EnableOAuth:                 false,
				APIKey:                      "api-key",
				APISecret:                   "api-secret",
				UsePreregisteredApplication: false,
				WebhookSecret:               "abcd",
			},
		},
		{
			description: "invalid configuration: API Keys app and pre-registered app enabled",
			config: &configuration{
				EnableOAuth:                 false,
				APIKey:                      "api-key",
				APISecret:                   "api-secret",
				UsePreregisteredApplication: true,
				WebhookSecret:               "abcd",
			},
			errMsg: "pre-registered application can work only with OAuth enabled",
		},
		{
			description: "invalid configuration: custom Zoom URL with pre-registered app",
			config: &configuration{
				EnableOAuth:                 true,
				UsePreregisteredApplication: true,
				ZoomURL:                     "https://myzooom.com",
				EncryptionKey:               "abcd",
				WebhookSecret:               "abcd",
			},
			errMsg: "pre-registered application can only be used with official Zoom's vendor-hosted SaaS service",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			err := testCase.config.IsValid()
			if testCase.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testCase.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
