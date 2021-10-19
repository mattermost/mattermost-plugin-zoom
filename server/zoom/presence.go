package zoom

type ParticipantJoinedLeftEvent struct {
	EventType string	`json:"event"`
	EventTimestamp int                  `json:"event_ts"`
	Payload        ParticipantJoinedLeftEventPayload `json:"payload"`
}

type ParticipantJoinedLeftEventPayload struct {
	AccountId string	`json:"account_id"`
	Object ParticipantJoinedLeftEventObject 	`json:"object"`
}

type MeetingType int
const (
	Prescheduled MeetingType 	= 0
	Instant						= 1
	Scheduled					= 2
	RecurringNoFixedTime		= 3
	PersonalMeetingRoom			= 4
	PersonalAudioConference		= 7
	RecurringFixedTime			= 8
)

type ParticipantJoinedLeftEventObject struct {
	Id string	`json:"id"`
	Uuid string	`json:"uuid"`
	HostId string	`json:"host_id"`
	Topic string	`json:"topic""`
	Type MeetingType	`json:"type"`
	StartTime string	`json:"start_time""`
	TimeZone string		`json:"timezone"`
	Duration int		`json:"duration"`
	Participant ParticipantJoinedLeftEventParticipant `json:"participant"`
}

type ParticipantJoinedLeftEventParticipant struct {
	UserId string `json:"user_id"`
	UserName string	`json:"user_name"`
	Id string	`json:"id"`
	LeaveTime string `json:"leave_time"`
	LeaveReason string	`json:"leave_reason"`
	Email string	`json:"email"`
	RegistrantId string	`json:"registrant_id"`
	ParticipantUserId string	`json:"participant_user_id"`
}