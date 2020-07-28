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

type OAuthClient struct {
	ZoomEmail string

	// Zoom OAuth Token, ttl 15 years
	OAuthToken *oauth2.Token

	// Mattermost userID
	UserID string

	// Zoom userID
	ZoomID string
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
		return nil, errors.New("error fetching zoom user")
	}

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(res.Body); err != nil {
		return nil, errors.New("error reading response body for zoom user")
	}

	if err := json.Unmarshal(buf.Bytes(), user); err != nil {
		return nil, errors.New("error unmarshalling zoom user")
	}

	return user, nil
}

func (u OAuthClient) GetMeetingViaOAuth(meetingID int, conf *oauth2.Config, zoomAPIURL string) (meeting *Meeting, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	client := conf.Client(ctx, u.OAuthToken)
	res, err := client.Get(fmt.Sprintf("%s/meetings/%v", zoomAPIURL, meetingID))
	if err != nil {
		return nil, errors.New("error fetching zoom user, err=" + err.Error())
	}
	if res == nil {
		return nil, errors.New("error fetching zoom user, empty result returned")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("error fetching zoom user")
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.New("error reading response body for zoom user")
	}

	if err := json.Unmarshal(buf, meeting); err != nil {
		return nil, errors.New("error unmarshalling zoom user")
	}

	return meeting, nil
}
