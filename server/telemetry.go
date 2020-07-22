package main

const (
	telemetryOauthModeJWT   = "JWT"
	telemetryOauthModeOauth = "Oauth"

	telemetryStartSourceWebapp  = "webapp"
	telemetryStartSourceCommand = "command"
)

func (p *Plugin) trackConnect(userID string) {
	p.tracker.TrackUserEvent("connect", userID, map[string]interface{}{})
}

func (p *Plugin) trackDisconnect(userID string) {
	p.tracker.TrackUserEvent("disconnect", userID, map[string]interface{}{})
}

func (p *Plugin) trackOAuthModeChange(method string) {
	p.tracker.TrackEvent("oauth_mode_change", map[string]interface{}{
		"method": method,
	})
}

func (p *Plugin) trackMeetingStart(userID, source string) {
	p.tracker.TrackUserEvent("start_meeting", userID, map[string]interface{}{
		"source": source,
	})
}

func (p *Plugin) trackMeetingDuplication(userID string) {
	p.tracker.TrackUserEvent("meeting_duplicated", userID, map[string]interface{}{})
}

func (p *Plugin) trackMeetingForced(userID string) {
	p.tracker.TrackUserEvent("meeting_forced", userID, map[string]interface{}{})
}
