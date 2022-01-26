// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"reflect"
	"strings"

	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	zoomDefaultURL    = "https://zoom.us"
	zoomDefaultAPIURL = "https://api.zoom.us/v2"
)

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *config.Configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &config.Configuration{}
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
func (p *Plugin) setConfiguration(configuration *config.Configuration) {
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
	var configuration = new(config.Configuration)
	prevConfigEnableOAuth := false
	if p.configuration != nil {
		prevConfigEnableOAuth = p.configuration.EnableOAuth
	}
	prevConfigAccountLevelOAuth := false
	if p.configuration != nil {
		prevConfigAccountLevelOAuth = p.configuration.AccountLevelApp
	}

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	if err := p.registerSiteURL(); err != nil {
		return errors.Wrap(err, "could not register site URL")
	}

	p.setConfiguration(configuration)
	p.jwtClient = zoom.NewJWTClient(p.getZoomAPIURL(), configuration.APIKey, configuration.APISecret)

	enableDiagnostics := false
	if config := p.API.GetConfig(); config != nil {
		if configValue := config.LogSettings.EnableDiagnostics; configValue != nil {
			enableDiagnostics = *configValue
		}
	}
	p.tracker = telemetry.NewTracker(p.telemetryClient, p.API.GetDiagnosticId(), p.API.GetServerVersion(), manifest.ID, manifest.Version, "zoom", enableDiagnostics)

	if prevConfigEnableOAuth != p.configuration.EnableOAuth {
		method := telemetryOauthModeJWT
		if p.configuration.EnableOAuth {
			method = telemetryOauthModeOauth
		}

		p.trackOAuthModeChange(method)
	}
	if prevConfigAccountLevelOAuth != p.configuration.AccountLevelApp {
		method := telemetryOauthModeOauth
		if p.configuration.AccountLevelApp {
			method = telemetryOauthModeOauthAccountLevel
		}

		p.trackOAuthModeChange(method)
	}

	// re-register the plugin command here as a configuration update might change the available commands
	command, err := p.getCommand()
	if err != nil {
		return errors.Wrap(err, "failed to get command")
	}

	err = p.API.RegisterCommand(command)
	if err != nil {
		return errors.Wrap(err, "failed to register command")
	}

	return nil
}

// getZoomURL gets the configured Zoom URL. Default URL is https://zoom.us
func (p *Plugin) getZoomURL() string {
	zoomURL := strings.TrimSpace(p.getConfiguration().ZoomURL)
	if zoomURL == "" {
		zoomURL = zoomDefaultURL
	}
	return zoomURL
}

// getZoomAPIURL gets the configured Zoom API URL. Default URL is https://api.zoom.us/v2.
func (p *Plugin) getZoomAPIURL() string {
	apiURL := strings.TrimSpace(p.getConfiguration().ZoomAPIURL)
	if apiURL == "" {
		apiURL = zoomDefaultAPIURL
	}
	return apiURL
}
