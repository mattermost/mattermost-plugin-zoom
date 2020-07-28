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

	zoomStateLength   = 3
	zoomOAuthMessage  = "[Click here to link your Zoom account.](%s/plugins/zoom/oauth2/connect?channelID=%s)"
	zoomEmailMismatch = "We could not verify your Mattermost account in Zoom. Please ensure that your Mattermost email address %s matches your Zoom login email address."

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

	p.apiClient = zoom.NewClient(p.getZoomAPIURL(), config.APIKey, config.APISecret)

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

	zoomURL := p.getZoomURL()
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

func (p *Plugin) storeZoomUserInfo(info *zoom.UserInfo) error {
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

func (p *Plugin) getZoomUserInfo(userID string) (*zoom.UserInfo, error) {
	infoBytes, appErr := p.API.KVGet(zoomTokenKey + userID)
	if appErr != nil || infoBytes == nil {
		return nil, errors.New("must connect user account to Zoom first")
	}

	var userInfo zoom.UserInfo
	err := json.Unmarshal(infoBytes, &userInfo)
	if err != nil {
		return nil, errors.New("unable to parse token")
	}

	config := p.getConfiguration()
	unencryptedToken, err := decrypt([]byte(config.EncryptionKey), userInfo.OAuthToken.AccessToken)
	if err != nil {
		log.Println(err.Error())
		return nil, errors.New("unable to decrypt access token")
	}

	userInfo.OAuthToken.AccessToken = unencryptedToken
	return &userInfo, nil
}

func (p *Plugin) authenticateAndFetchZoomUser(userID, userEmail, channelID string) (*zoom.User, *zoom.AuthError) {
	config := p.getConfiguration()

	// use OAuth if available
	if config.EnableOAuth {
		zoomUserInfo, err := p.getZoomUserInfo(userID)
		oauthMsg := fmt.Sprintf(
			zoomOAuthMessage,
			*p.API.GetConfig().ServiceSettings.SiteURL, channelID,
		)

		if err != nil {
			return nil, &zoom.AuthError{Message: oauthMsg, Err: err}
		}

		conf, err := p.getOAuthConfig()
		if err != nil {
			return nil, &zoom.AuthError{Message: oauthMsg, Err: err}
		}

		zoomUser, err := zoom.GetUserViaOAuth(zoomUserInfo.OAuthToken, conf, p.getZoomAPIURL())
		if err != nil {
			return nil, &zoom.AuthError{Message: oauthMsg, Err: err}
		}
		return zoomUser, nil
	}

	// use personal credentials if OAuth is not available
	zoomUser, err := p.apiClient.GetUser(userEmail)
	if err != nil {
		return nil, &zoom.AuthError{
			Message: fmt.Sprintf(zoomEmailMismatch, userEmail),
			Err:     err,
		}
	}
	return zoomUser, nil
}

func (p *Plugin) disconnect(userID string) error {
	rawInfo, appErr := p.API.KVGet(zoomTokenKey + userID)
	if appErr != nil {
		return appErr
	}

	var userInfo zoom.UserInfo
	if err := json.Unmarshal(rawInfo, &userInfo); err != nil {
		return err
	}

	errByMattermostID := p.API.KVDelete(zoomTokenKey + userID)
	errByZoomID := p.API.KVDelete(zoomTokenKeyByZoomID + userInfo.ZoomID)
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
