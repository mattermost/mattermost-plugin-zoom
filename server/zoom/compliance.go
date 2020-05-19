package zoom

import (
	"net/http"
)

type complianceRequest struct {
	ClientID                     string                 `json:"client_id"`
	UserID                       string                 `json:"user_id"`
	AccountID                    string                 `json:"account_id"`
	DeauthorizationEventReceived DeauthorizationPayload `json:"deauthorization_event_received"`
	ComplianceCompleted          bool                   `json:"compliance_completed"`
}

func (c *Client) CompleteCompliance(payload DeauthorizationPayload) *ClientError {
	req := complianceRequest{
		ClientID:                     payload.ClientID,
		UserID:                       payload.UserID,
		AccountID:                    payload.AccountID,
		DeauthorizationEventReceived: payload,
		ComplianceCompleted:          true,
	}

	var ret string

	return c.request(http.MethodPost, "/oauth/data/compliance", req, &ret)
}
