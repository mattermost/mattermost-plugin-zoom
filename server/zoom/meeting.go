// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"fmt"
	"net/http"
)

// The User object defined at https://marketplace.zoom.us/docs/api-reference/zoom-api/meetings/meeting.
type Meeting struct {
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
	StartURL          string `json:"start_url"`
	Password          string `json:"password"`
	H323Password      string `json:"h323_password"`
	EncryptedPassword string `json:"encrypted_password"`
	PMI               int    `json:"pmi"`
	TrackingFields    []struct {
		Field string `json:"field"`
		Value string `json:"value"`
	} `json:"tracking_fields"`
	Occurrences []struct {
		OccurrenceID string `json:"occurrence_id"`
		StartTime    string `json:"start_time"`
		Duration     int    `json:"duration"`
		Status       string `json:"status"`
	} `json:"occurrences"`
	Settings struct {
		HostVideo             bool     `json:"host_video"`
		ParticipantVideo      bool     `json:"participant_video"`
		CNMeeting             bool     `json:"cn_meeting"`
		INMeeting             bool     `json:"in_meeting"`
		JoinBeforeHost        bool     `json:"join_before_host"`
		MuteUponEntry         bool     `json:"mute_upon_entry"`
		Watermark             bool     `json:"watermark"`
		UsePMI                bool     `json:"use_pmi"`
		ApprovalType          int      `json:"approval_type"`
		RegistrationType      int      `json:"registration_type"`
		Audio                 string   `json:"audio"`
		AutoRecording         string   `json:"auto_recording"`
		AlternativeHosts      string   `json:"alternative_hosts"`
		WaitingRoom           bool     `json:"waiting_room"`
		GlobalDialInCountries []string `json:"global_dial_in_countries"`
		GlobalDialInNumbers   []struct {
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

type StartMeetingRequest struct {
	Topic          string `json:"topic"`
	Type           int    `json:"type"`
	StartTime      string `json:"start_time,omitempty"`
	Duration       int    `json:"duration,omitempty"`
	ScheduleFor    string `json:"schedule_for,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	Password       string `json:"password"`
	Agenda         string `json:"agenda"`
	TrackingFields []struct {
		Field string `json:"field"`
		Value string `json:"value"`
	} `json:"tracking_fields"`
	// Recurrence struct {
	// 	Type           int    `json:"type"`
	// 	RepeatInterval int    `json:"repeat_interval"`
	// 	WeeklyDays     string `json:"weekly_days,omitempty"`
	// 	MonthlyDay     int    `json:"monthly_day,omitempty"`
	// 	MonthlyWeekDay int    `json:"monthly_week_day,omitempty"`
	// 	EndTimes       int    `json:"end_times,omitempty"`
	// 	EndDateTime    int    `json:"end_date_time,omitempty"`
	// } `json:"recurrence,omitempty"`
	Settings struct {
		HostVideo             bool     `json:"host_video"`
		ParticipantVideo      bool     `json:"participant_video"`
		CNMeeting             bool     `json:"cn_meeting"`
		INMeeting             bool     `json:"in_meeting"`
		JoinBeforeHost        bool     `json:"join_before_host"`
		MuteUponEntry         bool     `json:"mute_upon_entry"`
		Watermark             bool     `json:"watermark"`
		UsePMI                bool     `json:"use_pmi"`
		ApprovalType          int      `json:"approval_type"`
		RegistrationType      int      `json:"registration_type"`
		Audio                 string   `json:"audio"`
		AutoRecording         string   `json:"auto_recording"`
		AlternativeHosts      string   `json:"alternative_hosts"`
		WaitingRoom           bool     `json:"waiting_room"`
		GlobalDialInCountries []string `json:"global_dial_in_countries"`
		GlobalDialInNumbers   []struct {
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
	} `json:"settings,omitempty"`
}

func (c *Client) GetMeeting(meetingID int) (*Meeting, *ClientError) {
	var ret Meeting
	err := c.request(http.MethodGet, fmt.Sprintf("/meetings/%v", meetingID), "", &ret)
	return &ret, err
}

func (c *Client) StartMeeting(userEmail string) (*Meeting, *ClientError) {
	var ret Meeting
	meetingRequest := StartMeetingRequest{
		Topic: "Meeting created on Mattermost",
		Type:  1,
	}
	err := c.request(http.MethodPost, fmt.Sprintf("/users/%s/meetings", userEmail), meetingRequest, &ret)
	fmt.Printf("DEBUG: %v\n", ret)
	return &ret, err
}
