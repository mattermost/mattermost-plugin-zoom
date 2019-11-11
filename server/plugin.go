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

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	postMeetingKey = "post_meeting_"

	botUserName    = "zoom"
	botDisplayName = "Zoom"
	botDescription = "Created by the Zoom plugin."

	zoomDefaultUrl    = "https://zoom.us"
	zoomDefaultAPIUrl = "https://api.zoom.com/v2"
	ZoomTokenKey      = "_zoomtoken"
	ZoomEmailKey      = "_zoomemail"

	zoomOAuthmessage  = "[Click here to link your Zoom account.](%s/plugins/zoom/oauth/connect?channelID=%s)"
	zoomEmailMismatch = "We could not verify your Mattermost account in Zoom. Please ensure that your Mattermost email address matches your Zoom login email address."
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
}

// OnActivate checks if the configurations is valid and ensures the bot account exists
func (p *Plugin) OnActivate() error {
	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
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

	return nil
}

func (p *Plugin) getOAuthConfig() (*oauth2.Config, error) {
	config := p.getConfiguration()

	if !config.enableOAuth() {
		return nil, errors.New("please set OAuthClientID, OAuthClientSecret and EncryptionKey")
	}

	clientID := config.OAuthClientID
	clientSecret := config.OAuthClientSecret
	zoomUrl := config.ZoomURL
	zoomAPIUrl := config.ZoomAPIURL

	if zoomUrl == "" {
		zoomUrl = zoomDefaultUrl
	}
	if zoomAPIUrl == "" {
		zoomAPIUrl = zoomDefaultAPIUrl
	}

	authUrl := fmt.Sprintf("%v/oauth/authorize", zoomUrl)
	tokenUrl := fmt.Sprintf("%v/oauth/token", zoomUrl)

	siteUrl := *p.API.GetConfig().ServiceSettings.SiteURL
	if siteUrl == "" {
		return nil, errors.New("error fetching siteUrl")
	}

	redirectUrl := fmt.Sprintf("%s/plugins/zoom/oauth/complete", siteUrl)

	// TODO remove this hard coded url
	redirectUrl = "https://aea67a23.ngrok.io/plugins/zoom/oauth/complete"

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authUrl,
			TokenURL: tokenUrl,
		},
		RedirectURL: redirectUrl,
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

	// Mattermorst userID
	UserID string
}

type AuthError struct {
	Message string `json:"message"`
	err     error  `json:"err"`
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

	if err := p.API.KVSet(info.UserID+ZoomTokenKey, jsonInfo); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) getZoomUserInfo(userID string) (*ZoomUserInfo, error) {
	config := p.getConfiguration()

	var userInfo ZoomUserInfo

	if infoBytes, err := p.API.KVGet(userID + ZoomTokenKey); err != nil || infoBytes == nil {
		return nil, errors.New("Must connect user account to GitHub first.")
	} else if err := json.Unmarshal(infoBytes, &userInfo); err != nil {
		return nil, errors.New("Unable to parse token.")
	}

	unencryptedToken, err := decrypt([]byte(config.EncryptionKey), userInfo.OAuthToken.AccessToken)
	if err != nil {
		log.Println(err.Error())
		return nil, errors.New("Unable to decrypt access token.")
	}

	userInfo.OAuthToken.AccessToken = unencryptedToken

	return &userInfo, nil
}

func (p *Plugin) storeZoomToUserIDMapping(zoomEmail, userID string) error {
	if err := p.API.KVSet(zoomEmail+ZoomEmailKey, []byte(userID)); err != nil {
		return fmt.Errorf("Encountered error saving github username mapping")
	}
	return nil
}

func (p *Plugin) getZoomToUserIDMapping(zoomEmail string) string {
	userID, _ := p.API.KVGet(zoomEmail + ZoomEmailKey)
	return string(userID)
}

func (p *Plugin) authenticateAndFetchZoomUser(userID, userEmail, channelID string) (*zoom.User, *AuthError) {
	var zoomUser *zoom.User
	var clientErr *zoom.ClientError
	var err error
	config := p.getConfiguration()

	// use OAuth
	if config.enableOAuth() {
		zoomUserInfo, apiErr := p.getZoomUserInfo(userID)
		oauthMsg := fmt.Sprintf(
			zoomOAuthmessage,
			*p.API.GetConfig().ServiceSettings.SiteURL, channelID)

		if apiErr != nil || zoomUserInfo == nil {
			return nil, &AuthError{Message: oauthMsg, err: apiErr}
		}
		zoomUser, err = p.getZoomUserWithToken(zoomUserInfo.OAuthToken)
		if err != nil || zoomUser == nil {
			return nil, &AuthError{Message: oauthMsg, err: apiErr}
		}
	} else if config.EnableLegacyAuth {
		// use personal credentials
		zoomUser, clientErr = p.zoomClient.GetUser(userEmail)
		if clientErr != nil {
			return nil, &AuthError{Message: zoomEmailMismatch, err: clientErr}
		}
	}
	return zoomUser, nil
}

func (p *Plugin) getZoomUserWithToken(token *oauth2.Token) (*zoom.User, error) {

	config := p.getConfiguration()
	ctx := context.Background()

	conf, err := p.getOAuthConfig()
	if err != nil {
		return nil, err
	}

	client := conf.Client(ctx, token)

	apiUrl := config.ZoomAPIURL
	if apiUrl == "" {
		apiUrl = zoomDefaultAPIUrl
	}

	url := fmt.Sprintf("%v/users/me", config.ZoomAPIURL)
	res, err := client.Get(url)
	if err != nil || res == nil {
		return nil, errors.New("error fetching zoom user")
	}

	defer closeBody(res)
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

	log.Println(zoomUser)

	return &zoomUser, nil
}
