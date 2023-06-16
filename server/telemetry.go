package main

const (
	telemetryOauthModeOauth             = "Oauth"
	telemetryOauthModeOauthAccountLevel = "Oauth Account Level"

	telemetryStartSourceWebapp  = "webapp"
	telemetryStartSourceCommand = "command"
)

func (p *Plugin) TrackEvent(event string, properties map[string]interface{}) {
	err := p.tracker.TrackEvent(event, properties)
	if err != nil {
		p.API.LogDebug("Error sending telemetry event", "event", event, "error", err.Error())
	}
}

func (p *Plugin) TrackUserEvent(event, userID string, properties map[string]interface{}) {
	err := p.tracker.TrackUserEvent(event, userID, properties)
	if err != nil {
		p.API.LogDebug("Error sending user telemetry event", "event", event, "error", err.Error())
	}
}

func (p *Plugin) trackConnect(userID string) {
	p.TrackUserEvent("connect", userID, map[string]interface{}{})
}

func (p *Plugin) trackDisconnect(userID string) {
	p.TrackUserEvent("disconnect", userID, map[string]interface{}{})
}

func (p *Plugin) trackOAuthModeChange(method string) {
	p.TrackEvent("oauth_mode_change", map[string]interface{}{
		"method": method,
	})
}

func (p *Plugin) trackMeetingStart(userID, source string) {
	p.TrackUserEvent("start_meeting", userID, map[string]interface{}{
		"source": source,
	})
}

func (p *Plugin) trackMeetingDuplication(userID string) {
	p.TrackUserEvent("meeting_duplicated", userID, map[string]interface{}{})
}

func (p *Plugin) trackMeetingForced(userID string) {
	p.TrackUserEvent("meeting_forced", userID, map[string]interface{}{})
}
