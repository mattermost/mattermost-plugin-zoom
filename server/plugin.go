// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sync"

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

	zoomTokenKey         = "zoomtoken_"
	zoomTokenKeyByZoomID = "zoomtokenbyzoomid_"

	zoomStateLength = 3

	meetingPostIDTTL = 60 * 60 * 24 // One day
)

type Plugin struct {
	plugin.MattermostPlugin

	apiClient *zoom.APIClient

	// botUserID of the created bot account.
	botUserID string

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	siteURL string
}

// OnActivate checks if the configurations is valid and ensures the bot account exists
func (p *Plugin) OnActivate() error {
	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		return err
	}

	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	if siteURL == nil || *siteURL == "" {
		return errors.New("error fetching siteUrl")
	}
	p.siteURL = *siteURL

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

	p.apiClient = zoom.NewAPIClient(p.getZoomAPIURL(), config.APIKey, config.APISecret)

	return nil
}

// getActiveClient returns an OAuth Zoom client if available, otherwise it returns the API client.
func (p *Plugin) getActiveClient(user *model.User, channelID string) (zoom.Client, error) {
	config := p.getConfiguration()

	if !config.EnableOAuth {
		return p.apiClient, nil
	}

	info, err := p.getOAuthInfo(user.Id)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Zoom OAuth info")
	}

	conf, err := p.getOAuthConfig()
	if err != nil {
		return nil, errors.Wrap(err, "could not get Zoom OAuth config")
	}

	return zoom.NewOAuthClient(info, conf, p.siteURL, channelID, p.getZoomAPIURL()), nil
}

// getOAuthConfig returns the Zoom OAuth2 flow configuration.
func (p *Plugin) getOAuthConfig() (*oauth2.Config, error) {
	config := p.getConfiguration()
	zoomURL := p.getZoomURL()

	return &oauth2.Config{
		ClientID:     config.OAuthClientID,
		ClientSecret: config.OAuthClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%v/oauth/authorize", zoomURL),
			TokenURL: fmt.Sprintf("%v/oauth/token", zoomURL),
		},
		RedirectURL: fmt.Sprintf("%s/plugins/zoom/oauth2/complete", p.siteURL),
		Scopes: []string{
			"user:read",
			"meeting:write",
			"webinar:write",
			"recording:write"},
	}, nil
}

func (p *Plugin) storeOAuthInfo(info *zoom.OAuthInfo) error {
	config := p.getConfiguration()

	encryptedToken, err := encrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		return errors.Wrap(err, "could not encrypt OAuth token")
	}
	info.OAuthToken.AccessToken = encryptedToken

	encoded, err := json.Marshal(info)
	if err != nil {
		return err
	}

	if err := p.API.KVSet(zoomTokenKey+info.UserID, encoded); err != nil {
		return err
	}

	if err := p.API.KVSet(zoomTokenKeyByZoomID+info.ZoomID, encoded); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) getOAuthInfo(userID string) (*zoom.OAuthInfo, error) {
	encoded, appErr := p.API.KVGet(zoomTokenKey + userID)
	if appErr != nil || encoded == nil {
		return nil, errors.New("must connect user account to Zoom first")
	}

	var info zoom.OAuthInfo
	if err := json.Unmarshal(encoded, &info); err != nil {
		return nil, errors.New("unable to parse token")
	}

	config := p.getConfiguration()
	unencryptedToken, err := decrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		log.Println(err.Error())
		return nil, errors.New("unable to decrypt access token")
	}

	info.OAuthToken.AccessToken = unencryptedToken
	return &info, nil
}

func (p *Plugin) authenticateAndFetchZoomUser(user *model.User, channelID string) (*zoom.User, *zoom.AuthError) {
	client, err := p.getActiveClient(user, channelID)
	if err != nil {
		return nil, &zoom.AuthError{
			Message: "could not get the active zoom client",
			Err:     err,
		}
	}

	return client.GetUser(user.Email)
}

func (p *Plugin) disconnect(userID string) error {
	encoded, appErr := p.API.KVGet(zoomTokenKey + userID)
	if appErr != nil {
		return appErr
	}

	var info zoom.OAuthInfo
	if err := json.Unmarshal(encoded, &info); err != nil {
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

func (p *Plugin) sendDirectMessage(userID string, message string) error {
	channel, err := p.API.GetDirectChannel(userID, p.botUserID)
	if err != nil {
		msg := "could not get bot's DM channel"
		p.API.LogInfo(msg, "user_id", userID)
		return errors.Wrap(err, msg)
	}

	post := &model.Post{
		Message:   message,
		ChannelId: channel.Id,
		UserId:    p.botUserID,
	}

	_, err = p.API.CreatePost(post)
	return err
}
