// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	postMeetingKey = "post_meeting_"

	botUserName    = "zoom"
	botDisplayName = "Zoom"
	botDescription = "Created by the Zoom plugin."

	zoomDefaultURL       = "https://zoom.us"
	zoomDefaultAPIURL    = "https://api.zoom.com/v2"
	zoomTokenKey         = "zoomtoken_"
	zoomTokenKeyByZoomID = "zoomtokenbyzoomid_"

	zoomStateLength   = 3
	zoomOAuthMessage  = "[Click here to link your Zoom account.](%s/plugins/zoom/oauth2/connect?channelID=%s)"
	zoomEmailMismatch = "We could not verify your Mattermost account in Zoom. Please ensure that your Mattermost email address %s matches your Zoom login email address."

	meetingPostIDTTL = 60 * 60 * 24 // One day
)

type Plugin struct {
	plugin.MattermostPlugin

	zoomClient *zoom.Client

	// botUserID of the created bot account.
	botUserID string

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	telemetryClient telemetry.Client
	tracker         telemetry.Tracker
}

// OnActivate checks if the configurations is valid and ensures the bot account exists
func (p *Plugin) OnActivate() error {
	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		return err
	}

	if _, err := p.getSiteURL(); err != nil {
		return err
	}

	botUserID, err := p.Helpers.EnsureBot(&model.Bot{
		Username:    botUserName,
		DisplayName: botDisplayName,
		Description: botDescription,
	})
	if err != nil {
		return errors.Wrap(err, "failed to ensure bot account")
	}
	p.botUserID = botUserID

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return errors.Wrap(err, "couldn't get bundle path")
	}

	if err = p.API.RegisterCommand(getCommand()); err != nil {
		return errors.WithMessage(err, "OnActivate: failed to register command")
	}

	profileImage, err := ioutil.ReadFile(filepath.Join(bundlePath, "assets", "profile.png"))
	if err != nil {
		return errors.Wrap(err, "couldn't read profile image")
	}

	if appErr := p.API.SetProfileImage(botUserID, profileImage); appErr != nil {
		return errors.Wrap(appErr, "couldn't set profile image")
	}

	p.zoomClient = zoom.NewClient(config.ZoomAPIURL, config.APIKey, config.APISecret)

	p.telemetryClient, err = telemetry.NewRudderClient()
	if err != nil {
		p.API.LogWarn("telemetry client not started", "error", err.Error())
	}
	return nil
}

func (p *Plugin) getSiteURL() (string, error) {
	siteURLRef := p.API.GetConfig().ServiceSettings.SiteURL
	if siteURLRef == nil || *siteURLRef == "" {
		return "", errors.New("error fetching siteUrl")
	}

	return *siteURLRef, nil
}

func (p *Plugin) getOAuthConfig() (*oauth2.Config, error) {
	config := p.getConfiguration()

	clientID := config.OAuthClientID
	clientSecret := config.OAuthClientSecret
	zoomURL := config.ZoomURL
	if zoomURL == "" {
		zoomURL = zoomDefaultURL
	}

	authURL := fmt.Sprintf("%v/oauth/authorize", zoomURL)
	tokenURL := fmt.Sprintf("%v/oauth/token", zoomURL)

	siteURL, err := p.getSiteURL()
	if err != nil {
		return nil, err
	}

	redirectURL := fmt.Sprintf("%s/plugins/zoom/oauth2/complete", siteURL)

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		RedirectURL: redirectURL,
		Scopes: []string{
			"user:read",
			"meeting:write",
			"webinar:write",
			"recording:write"},
	}, nil
}

type ZoomUserInfo struct {
	ZoomEmail string

	// Zoom OAuth Token, ttl 15 years
	OAuthToken *oauth2.Token

	// Mattermost userID
	UserID string

	// Zoom userID
	ZoomID string
}

type AuthError struct {
	Message string `json:"message"`
	Err     error  `json:"err"`
}

func (ae *AuthError) Error() string {
	errorString, _ := json.Marshal(ae)
	return string(errorString)
}

func (p *Plugin) storeZoomUserInfo(info *ZoomUserInfo) error {
	config := p.getConfiguration()

	encryptedToken, err := encrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		return err
	}

	info.OAuthToken.AccessToken = encryptedToken

	jsonInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}

	if err := p.API.KVSet(zoomTokenKey+info.UserID, jsonInfo); err != nil {
		return err
	}

	if err := p.API.KVSet(zoomTokenKeyByZoomID+info.ZoomID, jsonInfo); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) getZoomUserInfo(userID string) (*ZoomUserInfo, error) {
	config := p.getConfiguration()

	var userInfo ZoomUserInfo

	infoBytes, appErr := p.API.KVGet(zoomTokenKey + userID)
	if appErr != nil || infoBytes == nil {
		return nil, errors.New("must connect user account to Zoom first")
	}

	err := json.Unmarshal(infoBytes, &userInfo)
	if err != nil {
		return nil, errors.New("unable to parse token")
	}

	unencryptedToken, err := decrypt([]byte(config.EncryptionKey), userInfo.OAuthToken.AccessToken)
	if err != nil {
		log.Println(err.Error())
		return nil, errors.New("unable to decrypt access token")
	}

	userInfo.OAuthToken.AccessToken = unencryptedToken

	return &userInfo, nil
}

func (p *Plugin) authenticateAndFetchZoomUser(userID, userEmail, channelID string) (*zoom.User, *AuthError) {
	var zoomUser *zoom.User
	var clientErr *zoom.ClientError
	var err error
	config := p.getConfiguration()

	// use OAuth
	if config.EnableOAuth {
		zoomUserInfo, apiErr := p.getZoomUserInfo(userID)
		oauthMsg := fmt.Sprintf(
			zoomOAuthMessage,
			*p.API.GetConfig().ServiceSettings.SiteURL, channelID)

		if apiErr != nil || zoomUserInfo == nil {
			return nil, &AuthError{Message: oauthMsg, Err: apiErr}
		}
		zoomUser, err = p.getZoomUserWithToken(zoomUserInfo.OAuthToken)
		if err != nil || zoomUser == nil {
			return nil, &AuthError{Message: oauthMsg, Err: apiErr}
		}
	} else {
		// use personal credentials
		zoomUser, clientErr = p.zoomClient.GetUser(userEmail)
		if clientErr != nil {
			includeEmailInErr := fmt.Sprintf(zoomEmailMismatch, userEmail)
			return nil, &AuthError{Message: includeEmailInErr, Err: clientErr}
		}
	}
	return zoomUser, nil
}

func (p *Plugin) disconnect(userID string) error {
	rawInfo, appErr := p.API.KVGet(zoomTokenKey + userID)
	if appErr != nil {
		return appErr
	}

	var info ZoomUserInfo
	err := json.Unmarshal(rawInfo, &info)
	if err != nil {
		return err
	}

	errByMattermostID := p.API.KVDelete(zoomTokenKey + userID)
	errByZoomID := p.API.KVDelete(zoomTokenKeyByZoomID + info.ZoomID)
	if errByMattermostID != nil {
		return errByMattermostID
	}
	if errByZoomID != nil {
		return errByZoomID
	}
	return nil
}

func (p *Plugin) getZoomUserWithToken(token *oauth2.Token) (*zoom.User, error) {
	config := p.getConfiguration()
	ctx := context.Background()

	conf, err := p.getOAuthConfig()
	if err != nil {
		return nil, err
	}

	client := conf.Client(ctx, token)
	apiURL := config.ZoomAPIURL
	if apiURL == "" {
		apiURL = zoomDefaultAPIURL
	}

	url := fmt.Sprintf("%v/users/me", apiURL)
	res, err := client.Get(url)
	if err != nil || res == nil {
		return nil, errors.New("error fetching zoom user, err=" + err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("error fetching zoom user")
	}

	buf := new(bytes.Buffer)

	if _, err = buf.ReadFrom(res.Body); err != nil {
		return nil, errors.New("error reading response body for zoom user")
	}

	var zoomUser zoom.User

	if err := json.Unmarshal(buf.Bytes(), &zoomUser); err != nil {
		return nil, errors.New("error unmarshalling zoom user")
	}

	return &zoomUser, nil
}

func (p *Plugin) GetMeetingOAuth(meetingID int, userID string) (*zoom.Meeting, error) {
	config := p.getConfiguration()
	ctx, cancelFunct := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunct()

	conf, err := p.getOAuthConfig()
	if err != nil {
		return nil, err
	}

	zoomUserInfo, apiErr := p.getZoomUserInfo(userID)
	if apiErr != nil {
		return nil, apiErr
	}

	client := conf.Client(ctx, zoomUserInfo.OAuthToken)
	apiURL := config.ZoomAPIURL
	if apiURL == "" {
		apiURL = zoomDefaultAPIURL
	}

	url := fmt.Sprintf("%v/meetings/%v", apiURL, meetingID)
	res, err := client.Get(url)

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

	var ret zoom.Meeting

	if err := json.Unmarshal(buf, &ret); err != nil {
		return nil, errors.New("error unmarshalling zoom user")
	}

	return &ret, nil
}

func (p *Plugin) dm(userID string, message string) error {
	channel, err := p.API.GetDirectChannel(userID, p.botUserID)
	if err != nil {
		p.API.LogInfo("Couldn't get bot's DM channel", "user_id", userID)
		return err
	}

	post := &model.Post{
		Message:   message,
		ChannelId: channel.Id,
		UserId:    p.botUserID,
	}

	_, err = p.API.CreatePost(post)
	if err != nil {
		return err
	}
	return nil
}
