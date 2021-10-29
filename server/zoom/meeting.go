// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

// Meeting is defined at https://marketplace.zoom.us/docs/api-reference/zoom-api/meetings/meeting
type Meeting struct { // nolint: govet
	UUID              string `json:"uuid"`
	ID                int    `json:"id"`
	HostID            string `json:"host_id"`
	Topic             string `json:"topic"`
	Type              int    `json:"type"`
	Status            string `json:"status"`
	StartTime         string `json:"start_time"`
	Duration          int    `json:"duration"`
	Timezone          string `json:"timezone"`
	CreatedAt         string `json:"created_at"`
	Agenda            string `json:"agenda"`
	JoinURL           string `json:"join_url"`
	Password          string `json:"password"`
	H323Password      string `json:"h323_password"`
	EncryptedPassword string `json:"encrypted_password"`
	PMI               int    `json:"pmi"`
	TrackingFields    []struct {
		Field string `json:"field"`
		Value string `json:"value"`
	} `json:"tracking_fields"`
	Occurrences []struct { // nolint: govet
		OccurrenceID string `json:"occurrence_id"`
		StartTime    string `json:"start_time"`
		Duration     int    `json:"duration"`
		Status       string `json:"status"`
	} `json:"occurrences"`
	Settings struct { // nolint: govet
		HostVideo             bool       `json:"host_video"`
		ParticipantVideo      bool       `json:"participant_video"`
		CNMeeting             bool       `json:"cn_meeting"`
		INMeeting             bool       `json:"in_meeting"`
		JoinBeforeHost        bool       `json:"join_before_host"`
		MuteUponEntry         bool       `json:"mute_upon_entry"`
		Watermark             bool       `json:"watermark"`
		UsePMI                bool       `json:"use_pmi"`
		ApprovalType          int        `json:"approval_type"`
		RegistrationType      int        `json:"registration_type"`
		Audio                 string     `json:"audio"`
		AutoRecording         string     `json:"auto_recording"`
		AlternativeHosts      string     `json:"alternative_hosts"`
		WaitingRoom           bool       `json:"waiting_room"`
		GlobalDialInCountries []string   `json:"global_dial_in_countries"`
		GlobalDialInNumbers   []struct { // nolint: govet
			Country     string `json:"country"`
			CountryName string `json:"country_name"`
			City        string `json:"city"`
			Number      string `json:"number"`
			Type        string `json:"type"`
		} `json:"global_dial_in_numbers"`
		ContactName                  string `json:"contact_name"`
		ContactEmail                 string `json:"contact_email"`
		RegistrantsConfirmationEmail bool   `json:"registrants_confirmation_email"`
		RegistrantsEmailNotification bool   `json:"registrants_email_notification"`
		MeetingAuthentication        bool   `json:"meeting_authentication"`
		AuthenticationOption         string `json:"authentication_option"`
		AuthenticationDomains        string `json:"authentication_domains"`
		AuthenticationName           string `json:"authentication_name"`
	} `json:"settings"`
}
