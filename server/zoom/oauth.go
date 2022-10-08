// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	httpTimeout = time.Second * 10
	// OAuthPrompt stores the template to show the users to connect to Zoom
	OAuthPrompt = "[Click here to link your Zoom account.](%s/plugins/zoom/oauth2/connect)"
)

// OAuthUserInfo represents a Zoom user authenticated via OAuth.
type OAuthUserInfo struct {
	ZoomEmail  string
	OAuthToken *oauth2.Token // Zoom OAuth Token, ttl 15 years
	UserID     string        // Mattermost userID
	ZoomID     string        // Zoom userID
}

// OAuthClient represents an OAuth-based Zoom client.
type OAuthClient struct {
	token          *oauth2.Token
	config         *oauth2.Config
	siteURL        string
	apiURL         string
	isAccountLevel bool
	api            PluginAPI
}

// NewOAuthClient creates a new Zoom OAuthClient instance.
func NewOAuthClient(token *oauth2.Token, config *oauth2.Config, siteURL, apiURL string, isAccountLevel bool, api PluginAPI) Client {
	return &OAuthClient{token, config, siteURL, apiURL, isAccountLevel, api}
}

// GetUser returns the Zoom user via OAuth.
func (c *OAuthClient) GetUser(user *model.User, firstConnect bool) (*User, *AuthError) {
	zoomUser, err := c.getUserViaOAuth(user, firstConnect)
	if err != nil {
		if c.isAccountLevel {
			if err == errNotFound {
				return nil, &AuthError{fmt.Sprintf(zoomEmailMismatch, user.Email), err}
			}

			return nil, &AuthError{fmt.Sprintf("Error fetching user: %s", err), err}
		}

		return nil, &AuthError{fmt.Sprintf(OAuthPrompt, c.siteURL), err}
	}

	return zoomUser, nil
}

// GetMeeting returns the Zoom meeting with the given ID via OAuth.
func (c *OAuthClient) GetMeeting(meetingID int) (*Meeting, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	client := c.config.Client(ctx, c.token)
	res, err := client.Get(fmt.Sprintf("%s/meetings/%v", c.apiURL, meetingID))
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch zoom meeting")
	}
	if res == nil {
		return nil, errors.New("error fetching zoom meeting, empty result returned")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("%d error returned while fetching zoom meeting", res.StatusCode))
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "could not read response body for zoom meeting")
	}

	var meeting Meeting
	if err := json.Unmarshal(buf, &meeting); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal zoom meeting data")
	}

	return &meeting, nil
}

// CreateMeeting creates a new meeting for the user and returns the created meeting.
func (c *OAuthClient) CreateMeeting(user *User, topic string) (*Meeting, error) {
	client := c.config.Client(context.Background(), c.token)
	meetingRequest := CreateMeetingRequest{
		Topic: topic,
		Type:  MeetingTypeInstant,
	}
	b, err := json.Marshal(meetingRequest)
	if err != nil {
		return nil, err
	}

	res, err := client.Post(fmt.Sprintf("%s/users/%s/meetings", c.apiURL, user.Email), "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return nil, errors.New(res.Status)
	}

	var ret Meeting
	err = json.NewDecoder(res.Body).Decode(&ret)
	if err != nil {
		return nil, err
	}

	return &ret, err
}

func (c *OAuthClient) getUserViaOAuth(user *model.User, firstConnect bool) (*User, error) {
	url := fmt.Sprintf("%s/users/me", c.apiURL)
	if c.isAccountLevel {
		url = fmt.Sprintf("%s/users/%s", c.apiURL, user.Email)
	}

	if !firstConnect {
		if c.isAccountLevel {
			currentToken, err := c.api.GetZoomSuperUserToken()
			if err != nil {
				return nil, errors.Wrap(err, "error getting zoom super user token")
			}

			tokenSource := c.config.TokenSource(context.Background(), currentToken)
			updatedToken, err := tokenSource.Token()
			if err != nil {
				return nil, errors.Wrap(err, "error getting token from token source")
			}

			if updatedToken.RefreshToken != currentToken.RefreshToken {
				kvErr := c.api.SetZoomSuperUserToken(updatedToken)
				if kvErr != nil {
					return nil, errors.Wrap(kvErr, "error setting new token")
				}
			}

			c.token = updatedToken
		} else {
			info, err := c.api.GetZoomOAuthUserInfo(user.Id)
			if err != nil {
				return nil, errors.Wrap(err, "error getting zoom user token")
			}

			currentToken := info.OAuthToken

			tokenSource := c.config.TokenSource(context.Background(), currentToken)
			updatedToken, err := tokenSource.Token()
			if err != nil {
				return nil, errors.Wrap(err, "error getting token from token source")
			}

			if updatedToken.RefreshToken != currentToken.RefreshToken {
				info.OAuthToken = updatedToken
				kvErr := c.api.UpdateZoomOAuthUserInfo(user.Id, info)
				if kvErr != nil {
					return nil, errors.Wrap(kvErr, "error setting new token")
				}
			}

			c.token = updatedToken
		}
	}

	client := c.config.Client(context.Background(), c.token)

	res, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching zoom user")
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("%d error returned while fetching zoom user", res.StatusCode))
	}

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(res.Body); err != nil {
		return nil, errors.Wrap(err, "could not read response body for zoom user")
	}

	var zoomUser User
	if err := json.Unmarshal(buf.Bytes(), &zoomUser); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal zoom user data")
	}

	return &zoomUser, nil
}
