// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"
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

	trueString  = "true"
	falseString = "false"

	zoomProviderName = "Zoom"

	chimeraZoomUserLevelAppIdentifier    = "plugin-zoom-user-level"
	chimeraZoomAccountLevelAppIdentifier = "plugin-zoom-account-level"
)

type Plugin struct {
	plugin.MattermostPlugin

	jwtClient zoom.Client

	// botUserID of the created bot account.
	botUserID string

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	siteURL    string
	chimeraURL string

	telemetryClient telemetry.Client
	tracker         telemetry.Tracker
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

	p.registerChimeraURL()
	if config.UsePreregisteredApplication && p.chimeraURL == "" {
		return errors.New("cannot use pre-registered application if Chimera URL is not set or empty. " +
			"For now, using pre-registered application is intended for Cloud instances only. " +
			"If you are running on-prem, disable the setting and use a custom application, otherwise set PluginSettings.ChimeraOAuthProxyURL " +
			"or MM_PLUGINSETTINGS_CHIMERAOAUTHPROXYURL environment variable")
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

	p.telemetryClient, err = telemetry.NewRudderClient()
	if err != nil {
		p.API.LogWarn("telemetry client not started", "error", err.Error())
	}

	return nil
}

func (p *Plugin) OnDeactivate() error {
	if p.telemetryClient != nil {
		err := p.telemetryClient.Close()
		if err != nil {
			p.API.LogWarn("OnDeactivate: failed to close telemetryClient", "error", err.Error())
		}
	}

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

// registerChimeraURL fetches the Chimera URL from server settings or env var and sets it in the plugin object.
func (p *Plugin) registerChimeraURL() {
	chimeraURLSetting := p.API.GetConfig().PluginSettings.ChimeraOAuthProxyUrl
	if chimeraURLSetting != nil && *chimeraURLSetting != "" {
		p.chimeraURL = *chimeraURLSetting
		return
	}
	// Due to setting name change in v6 (ChimeraOAuthProxyUrl -> ChimeraOAuthProxyURL)
	// fall back to env var to work with new and older servers.
	p.chimeraURL = os.Getenv("MM_PLUGINSETTINGS_CHIMERAOAUTHPROXYURL")
}

// getActiveClient returns an OAuth Zoom client if available, otherwise it returns the API client.
func (p *Plugin) getActiveClient(user *model.User) (Client, string, error) {
	config := p.getConfiguration()

	// JWT
	if !config.EnableOAuth {
		return p.jwtClient, "", nil
	}

	// OAuth Account Level
	if config.AccountLevelApp {
		message := "Zoom App not connected. Contact your System administrator."
		token, err := p.getSuperuserToken()
		if user.IsSystemAdmin() {
			message = fmt.Sprintf(zoom.OAuthPrompt, p.siteURL)
		}
		if err != nil {
			return nil, message, errors.Wrap(err, "could not get token")
		}
		if token == nil {
			return nil, message, errors.New("zoom app not connected")
		}
		return zoom.NewOAuthClient(token, p.getOAuthConfig(), p.siteURL, p.getZoomAPIURL(), true, p), "", nil
	}

	// Oauth User Level
	message := fmt.Sprintf(zoom.OAuthPrompt, p.siteURL)
	info, err := p.fetchOAuthUserInfo(zoomUserByMMID, user.Id)
	if err != nil {
		return nil, message, errors.Wrap(err, "could not fetch Zoom OAuth info")
	}

	plainToken, err := decrypt([]byte(config.EncryptionKey), info.OAuthToken.AccessToken)
	if err != nil {
		return nil, message, errors.New("could not decrypt OAuth access token")
	}

	info.OAuthToken.AccessToken = plainToken
	conf := p.getOAuthConfig()
	return zoom.NewOAuthClient(info.OAuthToken, conf, p.siteURL, p.getZoomAPIURL(), false, p), "", nil
}

// getOAuthConfig returns the Zoom OAuth2 flow configuration.
func (p *Plugin) getOAuthConfig() *oauth2.Config {
	config := p.getConfiguration()
	redirectURL := fmt.Sprintf("%s/plugins/zoom/oauth2/complete", p.siteURL)

	if config.UsePreregisteredApplication {
		return p.getOAuthConfigForChimeraApp(redirectURL)
	}

	zoomURL := p.getZoomURL()

	return &oauth2.Config{
		ClientID:     config.OAuthClientID,
		ClientSecret: config.OAuthClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%v/oauth/authorize", zoomURL),
			TokenURL: fmt.Sprintf("%v/oauth/token", zoomURL),
		},
		RedirectURL: redirectURL,
	}
}

func (p *Plugin) getOAuthConfigForChimeraApp(redirectURL string) *oauth2.Config {
	baseURL := fmt.Sprintf("%s/v1/zoom/%s", p.chimeraURL, p.getZoomAppID())
	authURL, _ := url.Parse(baseURL)
	tokenURL, _ := url.Parse(baseURL)

	authURL.Path = path.Join(authURL.Path, "oauth", "authorize")
	tokenURL.Path = path.Join(tokenURL.Path, "oauth", "token")

	return &oauth2.Config{
		ClientID:     "placeholder",
		ClientSecret: "placeholder",
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:   authURL.String(),
			TokenURL:  tokenURL.String(),
			AuthStyle: oauth2.AuthStyleInHeader,
		},
	}
}

func (p *Plugin) getZoomAppID() string {
	if p.getConfiguration().AccountLevelApp {
		return chimeraZoomAccountLevelAppIdentifier
	}
	return chimeraZoomUserLevelAppIdentifier
}

// authenticateAndFetchZoomUser uses the active Zoom client to authenticate and return the Zoom user
func (p *Plugin) authenticateAndFetchZoomUser(user *model.User) (*zoom.User, *zoom.AuthError) {
	zoomClient, message, err := p.getActiveClient(user)
	if err != nil {
		return nil, &zoom.AuthError{
			Message: message,
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

func (p *Plugin) GetZoomSuperUserToken() (*oauth2.Token, error) {
	token, err := p.getSuperuserToken()
	if err != nil {
		return nil, errors.Wrap(err, "could not get token")
	}
	if token == nil {
		return nil, errors.New("zoom app not connected")
	}
	return token, nil
}

func (p *Plugin) SetZoomSuperUserToken(token *oauth2.Token) error {
	err := p.setSuperUserToken(token)
	if err != nil {
		return errors.Wrap(err, "could not set token")
	}
	return nil
}
