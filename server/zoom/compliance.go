// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package zoom

type ComplianceRequest struct {
	ClientID                     string                 `json:"client_id"`
	UserID                       string                 `json:"user_id"`
	AccountID                    string                 `json:"account_id"`
	DeauthorizationEventReceived DeauthorizationPayload `json:"deauthorization_event_received"`
	ComplianceCompleted          bool                   `json:"compliance_completed"`
}
