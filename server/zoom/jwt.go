// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
)

const (
	jwlAlgorithm      = "HS256"
	zoomEmailMismatch = "We could not verify your Mattermost account in Zoom. Please ensure that your Mattermost email address %s matches your Zoom login email address."
)

// JWTClient represents a JWT-based Zoom API client.
type JWTClient struct {
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	baseURL    string
}

// NewJWTClient returns a new JWT-based Zoom API client.
func NewJWTClient(zoomAPIURL, apiKey, apiSecret string) Client {
	return &JWTClient{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		httpClient: &http.Client{},
		baseURL:    zoomAPIURL,
	}
}

// GetMeeting returns the Zoom meeting with the given ID via JWT authentication.
func (c *JWTClient) GetMeeting(meetingID int) (*Meeting, error) {
	var meeting Meeting
	err := c.request(http.MethodGet, fmt.Sprintf("/meetings/%v", meetingID), "", &meeting)
	return &meeting, err
}

// GetUser returns the Zoom user via JWT authentication.
func (c *JWTClient) GetUser(user *model.User, firstConnect bool) (*User, *AuthError) {
	var zoomUser User
	if err := c.request(http.MethodGet, fmt.Sprintf("/users/%v", user.Email), "", &zoomUser); err != nil {
		return nil, &AuthError{fmt.Sprintf(zoomEmailMismatch, user.Email), err}
	}

	return &zoomUser, nil
}

// CreateMeeting creates a new meeting for the user and returns the created meeting.
func (c *JWTClient) CreateMeeting(user *User, topic string) (*Meeting, error) {
	var ret Meeting
	meetingRequest := CreateMeetingRequest{
		Topic: topic,
		Type:  MeetingTypeInstant,
	}
	err := c.request(http.MethodPost, fmt.Sprintf("/users/%s/meetings", user.Email), meetingRequest, &ret)
	return &ret, err
}

func (c *JWTClient) generateJWT() (string, error) {
	claims := jwt.MapClaims{}
	claims["iss"] = c.apiKey
	claims["exp"] = model.GetMillis() + (10 * 1000) // expire after 10s

	alg := jwt.GetSigningMethod(jwlAlgorithm)
	if alg == nil {
		return "", errors.New("couldn't find signing method")
	}

	token := jwt.NewWithClaims(alg, claims)
	return token.SignedString([]byte(c.apiSecret))
}

func (c *JWTClient) request(method, path string, data, ret interface{}) error {
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

	return json.Unmarshal(buf.Bytes(), &ret)
}
