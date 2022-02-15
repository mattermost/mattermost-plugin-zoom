package main

import (
	"strings"

	"github.com/pkg/errors"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
)

const (
	telemetryOauthModeJWT               = "JWT"
	telemetryOauthModeOauth             = "Oauth"
	telemetryOauthModeOauthAccountLevel = "Oauth Account Level"

	telemetryStartSourceWebapp  = "webapp"
	telemetryStartSourceCommand = "command"

	keysPerPage = 100
)

func (p *Plugin) trackConnect(userID string) {
	_ = p.tracker.TrackUserEvent("connect", userID, map[string]interface{}{})
}

func (p *Plugin) trackDisconnect(userID string) {
	_ = p.tracker.TrackUserEvent("disconnect", userID, map[string]interface{}{})
}

func (p *Plugin) trackOAuthModeChange(method string) {
	_ = p.tracker.TrackEvent("oauth_mode_change", map[string]interface{}{
		"method": method,
	})
}

func (p *Plugin) trackMeetingStart(userID, source string) {
	_ = p.tracker.TrackUserEvent("start_meeting", userID, map[string]interface{}{
		"source": source,
	})
}

func (p *Plugin) trackMeetingDuplication(userID string) {
	_ = p.tracker.TrackUserEvent("meeting_duplicated", userID, map[string]interface{}{})
}

func (p *Plugin) trackMeetingForced(userID string) {
	_ = p.tracker.TrackUserEvent("meeting_forced", userID, map[string]interface{}{})
}

func (p *Plugin) sendDailyTelemetry() {

	config := p.getConfiguration()

	connectedUserCount, err := p.getConnectedUserCount()
	if err != nil {
		p.API.LogWarn("Failed to get the number of connected users for telemetry", "error", err)
	}

	isJWT := !config.EnableOAuth

	_ = p.tracker.TrackEvent("stats", map[string]interface{}{
		"connected_user_count": connectedUserCount,
		"is_jwt_app":           isJWT,
		"is_jwt_configured":    config.APIKey != "" && config.APISecret != "",
		"is_user_level_app":    !isJWT && !config.AccountLevelApp,
		"is_account_level_app": !isJWT && config.AccountLevelApp,
		"is_oauth_configured":  config.OAuthClientID != "" && config.OAuthClientSecret != "",
		"use_vanity_url":       config.ZoomURL != "",
	})
}

func (p *Plugin) getConnectedUserCount() (int64, error) {
	checker := func(key string) (keep bool, err error) {
		return strings.HasSuffix(key, zoomUserByMMID), nil
	}

	var count int64

	for i := 0; ; i++ {
		keys, err := p.client.KV.ListKeys(i, keysPerPage, pluginapi.WithChecker(checker))
		if err != nil {
			return 0, errors.Wrapf(err, "failed to list keys - page, %d", i)
		}

		count += int64(len(keys))

		if len(keys) < keysPerPage {
			break
		}
	}

	return count, nil
}
