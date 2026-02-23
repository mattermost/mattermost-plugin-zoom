// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package zoom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	httpTimeout = time.Second * 10
	// OAuthPrompt stores the template to show the users to connect to Zoom
	OAuthPrompt       = "[Click here to link your Zoom account.](%s/plugins/zoom/oauth2/connect)"
	zoomEmailMismatch = "We could not verify your Mattermost account in Zoom. Please ensure that your Mattermost email address %s matches your Zoom login email address."
)

var ErrFetchingUser = errors.New("error returned while fetching Zoom user")

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

		if errors.Is(err, ErrFetchingUser) {
			return nil, &AuthError{"Error fetching user from Zoom", err}
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
		return nil, errors.Wrap(err, "could not fetch Zoom meeting")
	}
	if res == nil {
		return nil, errors.New("error fetching Zoom meeting, empty result returned")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d error returned while fetching Zoom meeting", res.StatusCode)
	}

	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "could not read response body for Zoom meeting")
	}

	var meeting Meeting
	if err := json.Unmarshal(buf, &meeting); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal Zoom meeting data")
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

	urlStr := fmt.Sprintf("%s/users/%s/meetings", c.apiURL, url.PathEscape(user.Email))
	res, err := client.Post(urlStr, "application/json", bytes.NewReader(b))
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
	urlStr := fmt.Sprintf("%s/users/me", c.apiURL)
	if c.isAccountLevel {
		urlStr = fmt.Sprintf("%s/users/%s", c.apiURL, url.PathEscape(user.Email))
	}

	if !firstConnect {
		if c.isAccountLevel {
			currentToken, err := c.api.GetZoomSuperUserToken()
			if err != nil {
				return nil, errors.Wrap(err, "error getting Zoom super user token")
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
				return nil, errors.Wrap(err, "error getting Zoom user token")
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

	res, err := client.Get(urlStr)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching Zoom user")
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d %w", res.StatusCode, ErrFetchingUser)
	}

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(res.Body); err != nil {
		return nil, errors.Wrap(err, "could not read response body for Zoom user")
	}

	var zoomUser User
	if err := json.Unmarshal(buf.Bytes(), &zoomUser); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal Zoom user data")
	}

	return &zoomUser, nil
}

func (c *OAuthClient) OpenDialogRequest(body *model.OpenDialogRequest) error {
	postURL := fmt.Sprintf("%s%s", c.siteURL, "/api/v4/actions/dialogs/open")
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	client := c.config.Client(context.Background(), c.token)
	res, err := client.Post(postURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}
