// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
)

const (
	zoomAPIKey     = "api.zoom.us"
	zoomAPIVersion = "v2"
	jwlAlgorithm   = "HS256"
)

type ClientError struct {
	StatusCode int
	Err        error
}

func (ce *ClientError) Error() string {
	return ce.Err.Error()
}

// Client represents a Zoom API client
type Client struct {
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	baseURL    string
}

// NewClient returns a new Zoom API client. An empty url will default to https://api.zoom.us/v2.
func NewClient(zoomURL, apiKey, apiSecret string) *Client {
	if zoomURL == "" {
		zoomURL = (&url.URL{
			Scheme: "https",
			Host:   zoomAPIKey,
			Path:   "/" + zoomAPIVersion,
		}).String()
	}

	return &Client{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		httpClient: &http.Client{},
		baseURL:    zoomURL,
	}
}

func (c *Client) generateJWT() (string, error) {
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

func (c *Client) request(method, path string, data, ret interface{}) *ClientError {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return &ClientError{http.StatusInternalServerError, err}
	}

	rq, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(jsonData))
	if err != nil {
		return &ClientError{http.StatusInternalServerError, err}
	}
	rq.Header.Set("Content-Type", "application/json")
	rq.Close = true

	token, err := c.generateJWT()
	if err != nil {
		return &ClientError{http.StatusInternalServerError, err}
	}
	rq.Header.Set("Authorization", "BEARER "+token)

	rp, err := c.httpClient.Do(rq)
	if err != nil {
		return &ClientError{
			http.StatusInternalServerError,
			errors.WithMessagef(err, "Unable to make request to %v", c.baseURL+path),
		}
	}

	if rp == nil {
		return &ClientError{
			http.StatusInternalServerError,
			errors.Errorf("Received nil response when making request to %v", c.baseURL+path),
		}
	}
	defer rp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(rp.Body); err != nil {
		return &ClientError{
			http.StatusInternalServerError,
			errors.Errorf("Failed to read response from %v", c.baseURL+path),
		}
	}

	if rp.StatusCode >= 300 {
		return &ClientError{rp.StatusCode, errors.New(buf.String())}
	}

	if err := json.Unmarshal(buf.Bytes(), &ret); err != nil {
		return &ClientError{rp.StatusCode, err}
	}

	return nil
}
