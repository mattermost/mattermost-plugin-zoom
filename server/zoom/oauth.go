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

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	httpTimeout = time.Second * 10
	OAuthPrompt = "[Click here to link your Zoom account.](%s/plugins/zoom/oauth2/connect?channelID=%s)"
)

type OAuthInfo struct {
	ZoomEmail  string
	OAuthToken *oauth2.Token // Zoom OAuth Token, ttl 15 years
	UserID     string        // Mattermost userID
	ZoomID     string        // Zoom userID
}

type OAuthClient struct {
	info      *OAuthInfo
	config    *oauth2.Config
	siteURL   string
	channelID string
	apiURL    string
}

func NewOAuthClient(info *OAuthInfo, config *oauth2.Config, siteURL, channelID, apiURL string) *OAuthClient {
	return &OAuthClient{info, config, siteURL, channelID, apiURL}
}

func (c *OAuthClient) GetUser(userID string) (*User, *AuthError) {
	user, err := GetUserViaOAuth(c.info.OAuthToken, c.config, c.apiURL)
	if err != nil {
		return nil, &AuthError{fmt.Sprintf(OAuthPrompt, c.siteURL, c.channelID), err}
	}

	return user, nil
}

func (c *OAuthClient) GetMeeting(meetingID int) (*Meeting, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	client := c.config.Client(ctx, c.info.OAuthToken)
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

func GetUserViaOAuth(token *oauth2.Token, conf *oauth2.Config, zoomAPIURL string) (user *User, err error) {
	client := conf.Client(context.Background(), token)
	url := fmt.Sprintf("%s/users/me", zoomAPIURL)
	res, err := client.Get(url)
	if err != nil || res == nil {
		return nil, errors.Wrap(err, "error fetching zoom user")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("%d error returned while fetching zoom user", res.StatusCode))
	}

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(res.Body); err != nil {
		return nil, errors.Wrap(err, "could not read response body for zoom user")
	}

	if err := json.Unmarshal(buf.Bytes(), user); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal zoom user data")
	}

	return user, nil
}
