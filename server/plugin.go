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
	postMeetingKey  = "post_meeting_"
	oldStatusKey    = "old_status_"
	changeStatusKey = "zoom_status_change"
  
	botUserName    = "zoom"
	botDisplayName = "Zoom"
	botDescription = "Created by the Zoom plugin."

	zoomProviderName = "Zoom"
	pluginURLPath    = "/plugins/" + botUserName
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

func (p *Plugin) setUserStatus(userID string, meetingID int, meetingEnd bool) error {
	storedStatusPref, appErr := p.API.KVGet(fmt.Sprintf("%v_%v", changeStatusKey, userID))
	if appErr != nil {
		p.API.LogDebug("Could not get stored status preference from KV ", appErr)
	}

	if storedStatusPref == nil {
		err := p.sendStatusChangeAttachment(userID, p.botUserID, meetingID)
		if err != nil {
			p.API.LogDebug("could not send status change attachment ", "error", err)
		}
	}

	if string(storedStatusPref) != yes {
		return nil
	}

	statusKey := fmt.Sprintf("%v%v", oldStatusKey, meetingID)
	if meetingEnd {
		statusVal, err := p.API.KVGet(statusKey)
		if err != nil {
			p.API.LogDebug("Could not get old status from KVStore", "err", appErr.Error())
			return err
		}

		newStatus := string(statusVal)
		if newStatus == "" {
			newStatus = model.STATUS_ONLINE
		}

		_, appErr = p.API.UpdateUserStatus(userID, newStatus)
		if appErr != nil {
			p.API.LogDebug("Could not get update status", "err", appErr.Error())
			return appErr
		}

		return nil
	}

	currentStatus, appErr := p.API.GetUserStatus(userID)
	if appErr != nil {
		p.API.LogDebug("Failed to update user status", "err", appErr)
		return appErr
	}

	oldStatus := ""
	if currentStatus.Manual {
		oldStatus = currentStatus.Status
	}

	appErr = p.API.KVSetWithExpiry(statusKey, []byte(oldStatus), meetingPostIDTTL)
	if appErr != nil {
		p.API.LogDebug("failed to store old status", "err", appErr)
		return appErr
	}

	_, appErr = p.API.UpdateUserStatus(userID, model.STATUS_DND)
	if appErr != nil {
		p.API.LogDebug("Failed to update user status", "err", appErr)
		return appErr
	}

	return nil
}

func (p *Plugin) sendStatusChangeAttachment(userID, botUserID string, meetingID int) error {
	url := pluginURLPath + postActionPath
	actionYes := &model.PostAction{
		Name: "Yes",
		Integration: &model.PostActionIntegration{
			URL: url,
			Context: map[string]interface{}{
				ContextAccept:    true,
				ContextMeetingID: meetingID,
			},
		},
	}

	actionNo := &model.PostAction{
		Name: "No",
		Integration: &model.PostActionIntegration{
			URL: url,
			Context: map[string]interface{}{
				ContextAccept:    false,
				ContextMeetingID: meetingID,
			},
		},
	}

	sa := &model.SlackAttachment{
		Title:   "Status change",
		Text:    "Allow Zoom plugin to automatically change status",
		Actions: []*model.PostAction{actionYes, actionNo},
	}

	attachmentPost := model.Post{}
	model.ParseSlackAttachment(&attachmentPost, []*model.SlackAttachment{sa})
	directChannel, appErr := p.API.GetDirectChannel(userID, botUserID)
	if appErr != nil {
		p.API.LogDebug("Create Attachment: ", appErr)
		return appErr
	}
	attachmentPost.ChannelId = directChannel.Id
	attachmentPost.UserId = botUserID

	_, appErr = p.API.CreatePost(&attachmentPost)
	if appErr != nil {
		p.API.LogDebug("Create Attachment: ", appErr)
		return appErr
	}

	return nil
}
