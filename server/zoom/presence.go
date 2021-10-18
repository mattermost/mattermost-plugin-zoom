package zoom

type PresenceEvent struct {
	EventType string	`json:"event"`
	EventTimestamp int                  `json:"event_ts"`
	Payload        PresenceEventPayload `json:"payload"`
}

type PresenceEventPayload struct {
	AccountId string              `json:"account_id"`
	Object    PresenceEventObject `json:"object"`
}

type PresenceEventObject struct {
	DateTime string	`json:"date_time"`
	Email string	`json:"email"`
	Id string		`json:"id"`
	PresenceStatus string	`json:"presence_status"`
}


