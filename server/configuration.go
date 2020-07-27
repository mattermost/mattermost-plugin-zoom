// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"reflect"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type configuration struct {
	ZoomURL           string
	ZoomAPIURL        string
	APIKey            string
	APISecret         string
	EnableOAuth       bool
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURL  string
	EncryptionKey     string
	WebhookSecret     string
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

// IsValid checks if all needed fields are set.
func (c *configuration) IsValid() error {
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

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)
	p.zoomClient = zoom.NewClient(configuration.ZoomAPIURL, configuration.APIKey, configuration.APISecret)

	// re-register the plugin command here as a configuration update might change the available commands
	if err := p.API.RegisterCommand(p.getCommand()); err != nil {
		return errors.Wrap(err, "OnConfigurationChange: failed to register command")
	}

	return nil
}
