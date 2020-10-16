// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	botUserName    = "zoom"
	botDisplayName = "Zoom"
	botDescription = "Created by the Zoom plugin."

	zoomProviderName = "Zoom"
)

type Plugin struct {
	plugin.MattermostPlugin

	jwtClient *zoom.JWTClient

	// botUserID of the created bot account.
	botUserID string

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	siteURL string
}

// Client defines a common interface for the API and OAuth Zoom clients
type Client interface {
	GetMeeting(meetingID int) (*zoom.Meeting, error)
	GetUser(user *model.User) (*zoom.User, *zoom.AuthError)
}

// OnActivate checks if the configurations is valid and ensures the bot account exists
func (p *Plugin) OnActivate() error {
	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		return err
	}

	if err := p.registerSiteURL(); err != nil {
		return errors.Wrap(err, "could not register site URL")
	}

	command, err := p.getCommand()
	if err != nil {
		return errors.Wrap(err, "failed to get command")
	}

	err = p.API.RegisterCommand(command)
	if err != nil {
		return errors.Wrap(err, "failed to register command")
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

	profileImage, err := ioutil.ReadFile(filepath.Join(bundlePath, "assets", "profile.png"))
	if err != nil {
		return errors.Wrap(err, "couldn't read profile image")
	}

	if appErr := p.API.SetProfileImage(botUserID, profileImage); appErr != nil {
		return errors.Wrap(appErr, "couldn't set profile image")
	}

	p.jwtClient = zoom.NewJWTClient(p.getZoomAPIURL(), config.APIKey, config.APISecret)

	return nil
}

// registerSiteURL fetches the site URL and sets it in the plugin object.
func (p *Plugin) registerSiteURL() error {
	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	if siteURL == nil || *siteURL == "" {
		return errors.New("could not fetch siteURL")
	}

	p.siteURL = *siteURL
	return nil
}

// getActiveClient returns an OAuth Zoom client if available, otherwise it returns the API client.
func (p *Plugin) getActiveClient(user *model.User) (Client, error) {
	config := p.getConfiguration()

	if !config.EnableOAuth {
		return p.jwtClient, nil
	}

	info, err := p.fetchOAuthUserInfo(zoomUserByMMID, user.Id)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch Zoom OAuth info")
	}

	plainToken, err := decrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		return nil, errors.New("could not decrypt OAuth access token")
	}

	info.OAuthToken.AccessToken = plainToken
	conf := p.getOAuthConfig()
	return zoom.NewOAuthClient(info.OAuthToken, conf, p.siteURL, p.getZoomAPIURL()), nil
}

// getOAuthConfig returns the Zoom OAuth2 flow configuration.
func (p *Plugin) getOAuthConfig() *oauth2.Config {
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
	}
}

// authenticateAndFetchZoomUser uses the active Zoom client to authenticate and return the Zoom user
func (p *Plugin) authenticateAndFetchZoomUser(user *model.User) (*zoom.User, *zoom.AuthError) {
	zoomClient, err := p.getActiveClient(user)
	if err != nil {
		// this error will occur if the active client is the OAuth client and the user isn't connected
		return nil, &zoom.AuthError{
			Message: fmt.Sprintf(zoom.OAuthPrompt, p.siteURL),
			Err:     err,
		}
	}

	return zoomClient.GetUser(user)
}

func (p *Plugin) sendDirectMessage(userID string, message string) error {
	channel, err := p.API.GetDirectChannel(userID, p.botUserID)
	if err != nil {
		msg := fmt.Sprintf("could not get or create DM channel for bot with ID: %s", p.botUserID)
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
