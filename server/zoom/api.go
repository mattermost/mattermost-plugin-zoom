// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
)

const jwlAlgorithm = "HS256"

// APIClient represents a Zoom API client
type APIClient struct {
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	baseURL    string
}

// NewClient returns a new Zoom API client
func NewClient(zoomAPIURL, apiKey, apiSecret string) *APIClient {
	return &APIClient{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		httpClient: &http.Client{},
		baseURL:    zoomAPIURL,
	}
}

func (c *APIClient) CompleteCompliance(payload DeauthorizationPayload) error {
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

func (c *APIClient) GetMeeting(meetingID int) (meeting *Meeting, err error) {
	err = c.request(http.MethodGet, fmt.Sprintf("/meetings/%v", meetingID), "", meeting)
	return meeting, err
}

func (c *APIClient) GetUser(userID string) (user *User, err error) {
	err = c.request(http.MethodGet, fmt.Sprintf("/users/%v", userID), "", user)
	return user, err
}

func (c *APIClient) generateJWT() (string, error) {
	claims := jwt.MapClaims{}

	claims["iss"] = c.apiKey
	claims["exp"] = model.GetMillis() + (10 * 1000) // expire after 10s

	alg := jwt.GetSigningMethod(jwlAlgorithm)
	if alg == nil {
		return "", errors.New("couldn't find signing method")
	}

	token := jwt.NewWithClaims(alg, claims)

	out, err := token.SignedString([]byte(c.apiSecret))
	if err != nil {
		return "", err
	}

	return out, nil
}

func (c *APIClient) request(method, path string, data, ret interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "could not marshal JSON data")
	}

	rq, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(jsonData))
	if err != nil {
		return errors.Wrap(err, "could not create HTTP request")
	}
	rq.Header.Set("Content-Type", "application/json")
	rq.Close = true

	token, err := c.generateJWT()
	if err != nil {
		return errors.Wrap(err, "could not generate JWT")
	}

	rq.Header.Set("Authorization", "BEARER "+token)
	rp, err := c.httpClient.Do(rq)
	if err != nil {
		return errors.WithMessagef(err, "Unable to make request to %v", c.baseURL+path)
	}

	if rp == nil {
		return errors.Errorf("Received nil response when making request to %v", c.baseURL+path)
	}
	defer rp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(rp.Body); err != nil {
		return errors.Errorf("Failed to read response from %v", c.baseURL+path)
	}

	if rp.StatusCode >= 300 {
		return errors.New(buf.String())
	}

	if err := json.Unmarshal(buf.Bytes(), &ret); err != nil {
		return err
	}

	return nil
}
