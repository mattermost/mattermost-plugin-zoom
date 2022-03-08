// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/gorilla/mux"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
	"github.com/mattermost/mattermost-plugin-zoom/server/wizard"
	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	botUserName    = "zoom"
	botDisplayName = "Zoom"
	botDescription = "Created by the Zoom plugin."

	trueString  = "true"
	falseString = "false"

	zoomProviderName = "Zoom"
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
	configuration *config.Configuration

	siteURL string

	client *pluginapi.Client

	telemetryClient telemetry.Client
	tracker         telemetry.Tracker

	flowManager *wizard.FlowManager
	router      *mux.Router
}

// Client defines a common interface for the API and OAuth Zoom clients
type Client interface {
	GetMeeting(meetingID int) (*zoom.Meeting, error)
	GetUser(user *model.User, firstConnect bool) (*zoom.User, *zoom.AuthError)
}

// OnActivate checks if the configurations is valid and ensures the bot account exists
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	if err := p.registerSiteURL(); err != nil {
		return errors.Wrap(err, "could not register site URL")
	}

	err := p.setDefaultConfiguration()
	if err != nil {
		return errors.Wrap(err, "failed to set default configuration")
	}

	p.router = p.createRouter()

	command, err := p.getCommand()
	if err != nil {
		return errors.Wrap(err, "failed to get command")
	}

	err = p.API.RegisterCommand(command)
	if err != nil {
		return errors.Wrap(err, "failed to register command")
	}

	botUserID, err := p.client.Bot.EnsureBot(&model.Bot{
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

	p.telemetryClient, err = telemetry.NewRudderClient()
	if err != nil {
		p.API.LogWarn("telemetry client not started", "error", err.Error())
	}

	config := p.getConfiguration()
	p.jwtClient = zoom.NewJWTClient(p.getZoomAPIURL(), config.APIKey, config.APISecret)

	p.flowManager = p.NewFlowManager()

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

func (p *Plugin) setDefaultConfiguration() error {
	config := p.getConfiguration()

	changed, err := config.SetDefaults()
	if err != nil {
		return err
	}

	if changed {
		appErr := p.API.SavePluginConfig(config.ToMap())
		if appErr != nil {
			return appErr
		}
	}

	return nil
}

func (p *Plugin) OnInstall(c *plugin.Context, event model.OnInstallEvent) error {
	// Don't start wizard if plugin is configured
	if p.getConfiguration().IsValid() == nil {
		return nil
	}

	return p.flowManager.StartConfigurationWizard(event.UserId)
}

func (p *Plugin) OnSendDailyTelemetry() {
	p.sendDailyTelemetry()
}

func (p *Plugin) NewFlowManager() *wizard.FlowManager {
	return wizard.NewFlowManager(
		p.getConfiguration,
		p.client,
		p.tracker,
		p.router,
		p.siteURL+"/plugins/zoom",
		p.botUserID,
	)
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

	conf := p.getOAuthConfig()
	return zoom.NewOAuthClient(info.OAuthToken, conf, p.siteURL, p.getZoomAPIURL(), false, p), "", nil
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
	}
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

	firstConnect := false
	return zoomClient.GetUser(user, firstConnect)
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

func (p *Plugin) GetZoomOAuthUserInfo(userID string) (*zoom.OAuthUserInfo, error) {
	info, err := p.fetchOAuthUserInfo(zoomUserByMMID, userID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get token")
	}
	if info == nil {
		return nil, errors.New("zoom app not connected")
	}

	return info, nil
}

func (p *Plugin) UpdateZoomOAuthUserInfo(userID string, info *zoom.OAuthUserInfo) error {
	if err := p.storeOAuthUserInfo(info); err != nil {
		msg := "unable to update user token"
		p.API.LogWarn(msg, "error", err.Error())
		return errors.Wrap(err, msg)
	}

	return nil
}

func (p *Plugin) isAuthorizedSysAdmin(userID string) (bool, error) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return false, appErr
	}
	if !strings.Contains(user.Roles, "system_admin") {
		return false, nil
	}
	return true, nil
}
