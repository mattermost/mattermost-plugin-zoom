// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package config

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/pkg/errors"
)

const (
	zoomDefaultURL    = "https://zoom.us"
	zoomDefaultAPIURL = "https://api.zoom.us/v2"
)

// Configuration captures the plugin's external Configuration as exposed in the Mattermost server
// Configuration, as well as values computed from the Configuration. Any public fields will be
// deserialized from the Mattermost server Configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// Configuration can change at any time, access to the Configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the Configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your Configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type Configuration struct {
	ZoomURL           string
	ZoomAPIURL        string
	APIKey            string
	APISecret         string
	EnableOAuth       bool
	AccountLevelApp   bool
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURL  string
	EncryptionKey     string
	WebhookSecret     string
}

type GetConfigurationFunc func() *Configuration

func (c *Configuration) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"zoomurl":           c.ZoomURL,
		"zoomapiurl":        c.ZoomAPIURL,
		"apikey":            c.APIKey,
		"apisecret":         c.APISecret,
		"enableoauth":       c.EnableOAuth,
		"accountlevelapp":   c.AccountLevelApp,
		"oauthclientid":     c.OAuthClientID,
		"oauthclientsecret": c.OAuthClientSecret,
		"encryptionkey":     c.EncryptionKey,
		"webhooksecret":     c.WebhookSecret,
	}
}

func (c *Configuration) SetDefaults() (bool, error) {
	changed := false

	if c.EncryptionKey == "" {
		secret, err := generateSecret()
		if err != nil {
			return false, err
		}

		c.EncryptionKey = secret
		changed = true
	}

	if c.WebhookSecret == "" {
		secret, err := generateSecret()
		if err != nil {
			return false, err
		}

		c.WebhookSecret = secret
		changed = true
	}

	return changed, nil
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *Configuration) Clone() *Configuration {
	var clone = *c
	return &clone
}

// IsValid checks if all needed fields are set.
func (c *Configuration) IsValid() error {
	switch {
	case !c.EnableOAuth:
		switch {
		case len(c.APIKey) == 0:
			return errors.New("please configure APIKey")

		case len(c.APISecret) == 0:
			return errors.New("please configure APISecret")
		}
	case c.EnableOAuth:
		switch {
		case len(c.OAuthClientSecret) == 0:
			return errors.New("please configure OAuthClientSecret")

		case len(c.OAuthClientID) == 0:
			return errors.New("please configure OAuthClientID")

		case len(c.EncryptionKey) == 0:
			return errors.New("please generate EncryptionKey from Zoom plugin settings")
		}
	default:
		return errors.New("please select either OAuth or Password based authentication")
	}

	if len(c.WebhookSecret) == 0 {
		return errors.New("please configure WebhookSecret")
	}

	return nil
}

func generateSecret() (string, error) {
	b := make([]byte, 256)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	s := base64.RawStdEncoding.EncodeToString(b)

	s = s[:32]

	return s, nil
}
