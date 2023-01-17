package main

const (
	telemetryOauthModeJWT               = "JWT"
	telemetryOauthModeOauth             = "Oauth"
	telemetryOauthModeOauthAccountLevel = "Oauth Account Level"

	telemetryStartSourceWebapp  = "webapp"
	telemetryStartSourceCommand = "command"
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

func (p *Plugin) trackMeetingType(userID string, usePMI bool) {
	_ = p.tracker.TrackUserEvent("meeting_type", userID, map[string]interface{}{
		"use_pmi": usePMI,
	})
}

func (p *Plugin) trackMeetingDuplication(userID string) {
	_ = p.tracker.TrackUserEvent("meeting_duplicated", userID, map[string]interface{}{})
}

func (p *Plugin) trackMeetingForced(userID string) {
	_ = p.tracker.TrackUserEvent("meeting_forced", userID, map[string]interface{}{})
}
