// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	defaultMeetingTopic      = "Zoom Meeting"
	zoomOAuthUserStateLength = 4
	settingDataError         = "something went wrong while getting settings data"
	pathWebhook              = "/webhook"
	pathStartMeeting         = "/api/v1/meetings"
	pathConnectUser          = "/oauth2/connect"
	pathCompleteUserOAuth    = "/oauth2/complete"
	pathDeauthorizeUser      = "/deauthorization"
	pathUpdatePMI            = "/api/v1/updatePMI"
	pathAskPMI               = "/api/v1/askPMI"
	pathChannelPreference    = "/api/v1/channel-preference"
	yes                      = "Yes"
	no                       = "No"
	ask                      = "Ask"
	actionForContext         = "action"
	userIDForContext         = "userID"
	channelIDForContext      = "channelID"
	rootIDForContext         = "rootID"
	usePersonalMeetingID     = "USE PERSONAL MEETING ID"
	useAUniqueMeetingID      = "USE A UNIQUE MEETING ID"
	MattermostUserIDHeader   = "Mattermost-User-ID"

	RestrictPreference                  = "Restrict"
	AllowPreference                     = "Allow"
	DefaultChannelRestrictionPreference = "Default"

	zoomSettingsCommandMessage   = "You can set a default value for this in your user settings via `/zoom settings` command."
	askForMeetingType            = "Which meeting ID would you like to use for creating this meeting?"
	WebsocketEventMeetingStarted = "meeting_started"
)

var ZoomChannelPreferences = map[string]string{
	RestrictPreference:                  "restrict",
	AllowPreference:                     "allow",
	DefaultChannelRestrictionPreference: "default",
}

type startMeetingRequest struct {
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"`
	Topic     string `json:"topic"`
	UsePMI    string `json:"use_pmi"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type MeetingURLResponse struct {
	MeetingURL string `json:"meeting_url"`
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	config := p.getConfiguration()
	if err := config.IsValid(p.isCloudLicense()); err != nil {
		http.Error(w, "This plugin is not configured.", http.StatusNotImplemented)
		return
	}

	switch path := r.URL.Path; path {
	case pathWebhook:
		p.handleWebhook(w, r)
	case pathStartMeeting:
		p.handleStartMeeting(w, r)
	case pathConnectUser:
		p.connectUserToZoom(w, r)
	case pathCompleteUserOAuth:
		p.completeUserOAuthToZoom(w, r)
	case pathDeauthorizeUser:
		p.deauthorizeUser(w, r)
	case pathUpdatePMI:
		p.submitFormPMIForPreference(w, r)
	case pathAskPMI:
		p.submitFormPMIForMeeting(w, r)
	case pathChannelPreference:
		p.handleChannelPreference(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) submitFormPMIForMeeting(w http.ResponseWriter, r *http.Request) {
	response := &model.PostActionIntegrationResponse{}
	decoder := json.NewDecoder(r.Body)
	postActionIntegrationRequest := &model.PostActionIntegrationRequest{}
	if err := decoder.Decode(&postActionIntegrationRequest); err != nil {
		p.API.LogWarn("Error decoding PostActionIntegrationRequest params.", "Error", err.Error())
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to write the response", http.StatusInternalServerError)
		}
		return
	}

	action := postActionIntegrationRequest.Context[actionForContext].(string)
	userID := postActionIntegrationRequest.Context[userIDForContext].(string)
	channelID := postActionIntegrationRequest.Context[channelIDForContext].(string)
	rootID := postActionIntegrationRequest.Context[rootIDForContext].(string)

	slackAttachment := model.SlackAttachment{
		Text: fmt.Sprintf("You have selected `%s` to start the meeting.", action),
	}

	userPMISettingPref, err := p.getPMISettingData(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if userPMISettingPref == "" {
		val := trueString
		meetingIDType := "personal"
		if action == useAUniqueMeetingID {
			val = falseString
			meetingIDType = "unique"
		}

		if err := p.storeUserPreference(userID, val); err != nil {
			p.API.LogWarn("failed to update preferences for the user", "Error", err.Error())
			return
		}

		slackAttachment = model.SlackAttachment{
			Text: fmt.Sprintf("All future meetings will use `%s` meeting ID. Type `/zoom settings` to change your meeting ID preference", meetingIDType),
		}
	}

	post := &model.Post{
		ChannelId: channelID,
		UserId:    p.botUserID,
		Id:        postActionIntegrationRequest.PostId,
		CreateAt:  time.Now().Unix() * 1000,
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{&slackAttachment})
	p.API.UpdateEphemeralPost(userID, post)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogError("failed to write response", "Error", err.Error())
	}

	p.startMeeting(action, userID, channelID, rootID)
}

func (p *Plugin) startMeeting(action, userID, channelID, rootID string) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		p.API.LogWarn("failed to get the user from userID", "Error", appErr.Error())
		return
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
	if authErr != nil {
		p.API.LogWarn("failed to authenticate and fetch the Zoom user", "Error", appErr.Error())
		return
	}

	var meetingID int
	var createMeetingErr error
	createMeetingWithPMI := false
	if action == usePersonalMeetingID {
		createMeetingWithPMI = true
		meetingID = zoomUser.Pmi

		if meetingID <= 0 {
			meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, defaultMeetingTopic)
			if createMeetingErr != nil {
				p.API.LogWarn("failed to create the meeting", "Error", createMeetingErr.Error())
				return
			}
			p.sendEnableZoomPMISettingMessage(userID, channelID, rootID)
		}
	} else {
		meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, defaultMeetingTopic)
		if createMeetingErr != nil {
			p.API.LogWarn("failed to create the meeting", "Error", createMeetingErr.Error())
			return
		}
	}

	if postMeetingErr := p.postMeeting(user, meetingID, channelID, rootID, defaultMeetingTopic); postMeetingErr != nil {
		p.API.LogWarn("failed to post the meeting", "Error", postMeetingErr.Error())
		return
	}

	p.trackMeetingStart(userID, telemetryStartSourceCommand)
	p.trackMeetingType(userID, createMeetingWithPMI)
}

func (p *Plugin) submitFormPMIForPreference(w http.ResponseWriter, r *http.Request) {
	response := &model.PostActionIntegrationResponse{}
	decoder := json.NewDecoder(r.Body)
	postActionIntegrationRequest := &model.PostActionIntegrationRequest{}
	if err := decoder.Decode(&postActionIntegrationRequest); err != nil {
		p.API.LogWarn("Error decoding PostActionIntegrationRequest params.", "Error", err.Error())
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to write the response", http.StatusInternalServerError)
		}
		return
	}

	action := postActionIntegrationRequest.Context[actionForContext].(string)
	mattermostUserID := r.Header.Get(MattermostUserIDHeader)
	if mattermostUserID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	channel, err := p.API.GetDirectChannel(mattermostUserID, p.botUserID)
	if err != nil {
		p.API.LogWarn("failed to get the bot's DM channel", "Error", err.Error())
		return
	}
	slackAttachment := p.slackAttachmentToUpdatePMI(action)
	slackAttachment.Actions = nil // Remove action buttons once responded

	val := ""
	switch action {
	case ask:
		val = zoomPMISettingValueAsk
	case no:
		val = falseString
	default:
		val = trueString
	}

	if err := p.storeUserPreference(mattermostUserID, val); err != nil {
		p.API.LogWarn("failed to update preferences for the user", "Error", err.Error())
		return
	}

	post := &model.Post{
		ChannelId: channel.Id,
		UserId:    p.botUserID,
		Id:        postActionIntegrationRequest.PostId,
		CreateAt:  time.Now().Unix() * 1000,
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{slackAttachment})
	p.API.UpdateEphemeralPost(mattermostUserID, post)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		p.API.LogError("failed to write response", "Error", err.Error())
	}
}

func (p *Plugin) connectUserToZoom(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(MattermostUserIDHeader)
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	// fetch OAuth user state from the KV store that has been saved in '/zoom start' command handler
	state, appErr := p.fetchOAuthUserState(userID)
	if appErr != nil {
		http.Error(w, "missing stored state", http.StatusNotFound)
		return
	}

	cfg := p.getOAuthConfig()
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (p *Plugin) completeUserOAuthToZoom(w http.ResponseWriter, r *http.Request) {
	authedUserID := r.Header.Get(MattermostUserIDHeader)
	if authedUserID == "" {
		http.Error(w, "Not authorized, missing Mattermost user id", http.StatusUnauthorized)
		return
	}

	code := r.URL.Query().Get("code")
	if len(code) == 0 {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	storedState, appErr := p.fetchOAuthUserState(authedUserID)
	if appErr != nil {
		http.Error(w, "missing stored state", http.StatusNotFound)
		return
	}

	userID, channelID, justConnect, err := parseOAuthUserState(storedState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	if storedState != state {
		http.Error(w, "OAuth user state mismatch", http.StatusUnauthorized)
		return
	}

	if appErr = p.deleteUserState(userID); appErr != nil {
		p.API.LogWarn("failed to delete OAuth user state from KV store", "error", appErr.Error())
	}

	conf := p.getOAuthConfig()
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		p.API.LogWarn("failed to create the access token", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if p.configuration.AccountLevelApp {
		err = p.setSuperUserToken(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	client := zoom.NewOAuthClient(token, conf, p.siteURL, p.getZoomAPIURL(), p.configuration.AccountLevelApp, p)
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		http.Error(w, appErr.Error(), http.StatusInternalServerError)
		return
	}

	firstConnect := true
	zoomUser, authErr := client.GetUser(user, firstConnect)
	if authErr != nil {
		if p.configuration.AccountLevelApp && !justConnect {
			http.Error(w, "Connection completed but there was an error creating the meeting. "+authErr.Message, http.StatusInternalServerError)
			return
		}

		p.API.LogWarn("failed to get the user", "error", authErr.Error())
		http.Error(w, "Could not complete the connection: "+authErr.Message, http.StatusInternalServerError)
		return
	}

	if !p.configuration.AccountLevelApp {
		info := &zoom.OAuthUserInfo{
			ZoomEmail:  zoomUser.Email,
			ZoomID:     zoomUser.ID,
			UserID:     userID,
			OAuthToken: token,
		}

		if err = p.storeOAuthUserInfo(info); err != nil {
			msg := "Unable to connect the user to Zoom"
			p.API.LogWarn(msg, "error", err.Error())
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
	}

	p.trackConnect(userID)
	if justConnect {
		p.postEphemeral(userID, channelID, "", "Successfully connected to Zoom")
	} else {
		// Returning error might not be appropriate here as the main logic for this API is to connect users.
		if _, err := p.handleMeetingCreation(channelID, "", defaultMeetingTopic, user, zoomUser); err != nil {
			p.API.LogWarn("Error in creating meeting", "Error", err.Error())
		}
	}

	html := `
<!DOCTYPE html>
<html>
	<head>
		<script>
			window.close();
		</script>
	</head>
	<body>
		<p>Completed connecting to Zoom. Please close this window.</p>
	</body>
</html>
`

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		p.API.LogWarn("failed to write the response", "error", err.Error())
	}
}

func (p *Plugin) postMeeting(creator *model.User, meetingID int, channelID string, rootID string, topic string) error {
	meetingURL := p.getMeetingURL(creator, meetingID)

	if topic == "" {
		topic = defaultMeetingTopic
	}

	if !p.API.HasPermissionToChannel(creator.Id, channelID, model.PermissionCreatePost) {
		return errors.New("this channel is not accessible, you might not have permissions to write in this channel. Contact the administrator of this channel to find out if you have access permissions")
	}

	slackAttachment := model.SlackAttachment{
		Fallback: fmt.Sprintf("Video Meeting started at [%d](%s).\n\n[Join Meeting](%s)", meetingID, meetingURL, meetingURL),
		Title:    topic,
		Text:     fmt.Sprintf("Meeting ID: [%d](%s)\n\n[Join Meeting](%s)", meetingID, meetingURL, meetingURL),
	}

	post := &model.Post{
		UserId:    creator.Id,
		ChannelId: channelID,
		RootId:    rootID,
		Message:   "I have started a meeting",
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"attachments":              []*model.SlackAttachment{&slackAttachment},
			"meeting_id":               meetingID,
			"meeting_link":             meetingURL,
			"meeting_status":           zoom.WebhookStatusStarted,
			"meeting_personal":         false,
			"meeting_topic":            topic,
			"meeting_creator_username": creator.Username,
			"meeting_provider":         zoomProviderName,
		},
	}

	createdPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		return appErr
	}

	if appErr = p.storeMeetingPostID(meetingID, createdPost.Id); appErr != nil {
		p.API.LogDebug("failed to store post id", "error", appErr)
	}

	p.client.Frontend.PublishWebSocketEvent(
		WebsocketEventMeetingStarted,
		map[string]interface{}{
			"meeting_url": meetingURL,
		},
		&model.WebsocketBroadcast{UserId: creator.Id},
	)

	return nil
}

func (p *Plugin) askPreferenceForMeeting(userID, channelID, rootID string) {
	apiEndPoint := fmt.Sprintf("/plugins/%s%s", manifest.Id, pathAskPMI)

	userPMISettingPref, err := p.getPMISettingData(userID)
	if err != nil {
		p.API.LogDebug("failed to get user PMI setting value", "userID", userID, "error", err)
		return
	}

	pretext := zoomSettingsCommandMessage
	if userPMISettingPref == "" {
		pretext = ""
	}

	slackAttachment := model.SlackAttachment{
		Pretext: pretext,
		Title:   askForMeetingType,
		Actions: []*model.PostAction{
			{
				Id:    "WithPMI",
				Name:  usePersonalMeetingID,
				Type:  model.PostActionTypeButton,
				Style: "default",
				Integration: &model.PostActionIntegration{
					URL: apiEndPoint,
					Context: map[string]interface{}{
						actionForContext:    usePersonalMeetingID,
						userIDForContext:    userID,
						channelIDForContext: channelID,
						rootIDForContext:    rootID,
					},
				},
			},
			{
				Id:    "WithoutPMI",
				Name:  useAUniqueMeetingID,
				Type:  model.PostActionTypeButton,
				Style: "default",
				Integration: &model.PostActionIntegration{
					URL: apiEndPoint,
					Context: map[string]interface{}{
						actionForContext:    useAUniqueMeetingID,
						userIDForContext:    userID,
						channelIDForContext: channelID,
						rootIDForContext:    rootID,
					},
				},
			},
		},
	}

	post := &model.Post{
		ChannelId: channelID,
		UserId:    p.botUserID,
		RootId:    rootID,
	}
	model.ParseSlackAttachment(post, []*model.SlackAttachment{&slackAttachment})
	p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) getPMISettingData(userID string) (string, error) {
	preference, reqErr := p.getUserPreference(userID)
	if reqErr != nil {
		return "", errors.New(settingDataError)
	}

	return preference, nil
}

func (p *Plugin) handleStartMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req startMeetingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	restrict, err := p.isChannelRestrictedForMeetings(req.ChannelID)
	if err != nil {
		p.API.LogError("Unable to check channel preference", "ChannelID", req.ChannelID, "Error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if restrict {
		if err = json.NewEncoder(w).Encode(ErrorResponse{"Creating Zoom meeting is disabled for this channel."}); err != nil {
			p.API.LogWarn("failed to write the response", "error", err.Error())
		}
		return
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if _, appErr = p.API.GetChannelMember(req.ChannelID, userID); appErr != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
	if authErr != nil {
		if err = json.NewEncoder(w).Encode(MeetingURLResponse{MeetingURL: ""}); err != nil {
			p.API.LogWarn("failed to write the response", "error", err.Error())
		}

		// the user state will be needed later while connecting the user to Zoom via OAuth
		if appErr := p.storeOAuthUserState(userID, req.ChannelID, false); appErr != nil {
			p.API.LogWarn("failed to store user state")
		}

		p.postAuthenticationMessage(req.ChannelID, userID, authErr.Message)
		return
	}

	if r.URL.Query().Get("force") == "" {
		recentMeeting, recentMeetingLink, creatorName, provider, cpmErr := p.checkPreviousMessages(req.ChannelID)
		if cpmErr != nil {
			http.Error(w, cpmErr.Error(), cpmErr.StatusCode)
			return
		}

		if recentMeeting {
			if err = json.NewEncoder(w).Encode(MeetingURLResponse{MeetingURL: ""}); err != nil {
				p.API.LogWarn("failed to write the response", "error", err.Error())
			}
			p.postConfirm(recentMeetingLink, req.ChannelID, req.Topic, userID, req.RootID, creatorName, provider)
			return
		}
	}

	topic := req.Topic
	if topic == "" {
		topic = defaultMeetingTopic
	}

	meetingURL, err := p.handleMeetingCreation(req.ChannelID, req.RootID, topic, user, zoomUser)
	if err != nil {
		p.API.LogWarn("Error in creating meeting", "Error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if r.URL.Query().Get("force") != "" {
		p.trackMeetingForced(userID)
	}

	if _, err = w.Write([]byte(fmt.Sprintf(`{"meeting_url": "%s"}`, meetingURL))); err != nil {
		p.API.LogWarn("failed to write the response", "Error", err.Error())
	}
}

func (p *Plugin) handleChannelPreference(w http.ResponseWriter, r *http.Request) {
	submitRequest := &model.SubmitDialogRequest{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&submitRequest); err != nil {
		p.API.LogError("Error decoding dialog request", "Error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if submitRequest.UserId == "" {
		p.API.LogError("Invalid user ID", "UserID", submitRequest.UserId)
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	if !p.API.HasPermissionTo(submitRequest.UserId, model.PermissionManageSystem) {
		p.API.LogError("Unable to resolve request due to insufficient permissions", "UserID", submitRequest.UserId)
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	zoomChannelSettingsMapValue := ZoomChannelSettingsMapValue{
		Preference: fmt.Sprint(submitRequest.Submission["preference"]),
	}

	if err := zoomChannelSettingsMapValue.IsValid(); err != nil {
		p.API.LogError("Invalid request body", "Error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := p.storeZoomChannelSettings(submitRequest.ChannelId, zoomChannelSettingsMapValue); err != nil {
		p.API.LogError("Error setting channel preference", "Error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (p *Plugin) createMeetingWithoutPMI(user *model.User, zoomUser *zoom.User, topic string) (int, error) {
	client, _, err := p.getActiveClient(user)
	if err != nil {
		p.API.LogWarn("Error getting the client", "Error", err.Error())
		return -1, err
	}

	meeting, err := client.CreateMeeting(zoomUser, topic)
	if err != nil {
		p.API.LogWarn("Error creating the meeting", "Error", err.Error())
		return -1, err
	}

	return meeting.ID, nil
}

func (p *Plugin) getMeetingURL(user *model.User, meetingID int) string {
	defaultURL := fmt.Sprintf("%s/j/%v", p.getZoomURL(), meetingID)
	client, _, err := p.getActiveClient(user)
	if err != nil {
		p.API.LogWarn("could not get the active Zoom client", "error", err.Error())
		return defaultURL
	}

	meeting, err := client.GetMeeting(meetingID)
	if err != nil {
		p.API.LogDebug("failed to get meeting")
		return defaultURL
	}
	return meeting.JoinURL
}

func (p *Plugin) postConfirm(meetingLink string, channelID string, topic string, userID string, rootID string, creatorName string, provider string) *model.Post {
	message := "There is another recent meeting created on this channel."
	if provider != zoomProviderName {
		message = fmt.Sprintf("There is another recent meeting created on this channel with %s.", provider)
	}

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		RootId:    rootID,
		Message:   message,
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"type":                     "custom_zoom",
			"meeting_link":             meetingLink,
			"meeting_status":           zoom.RecentlyCreated,
			"meeting_personal":         false,
			"meeting_topic":            topic,
			"meeting_creator_username": creatorName,
			"meeting_provider":         provider,
		},
	}

	p.trackMeetingDuplication(userID)

	return p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) postAuthenticationMessage(channelID string, userID string, message string) *model.Post {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   message,
	}

	return p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) sendEnableZoomPMISettingMessage(userID, channelID, rootID string) {
	p.postEphemeral(userID, channelID, rootID, "The meeting below is created with an unique meeting ID, to use Personal Meeting ID (PMI) for creating the meeting, you need to `Enable Personal Meeting ID` from your [Zoom settings](https://zoom.us/profile/setting).")
}

func (p *Plugin) postEphemeral(userID, channelID, rootID, message string) *model.Post {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		RootId:    rootID,
		Message:   message,
	}

	return p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) checkPreviousMessages(channelID string) (recentMeeting bool, meetingLink string, creatorName string, provider string, err *model.AppError) {
	var zoomMeetingTimeWindow int64 = 30 // 30 seconds

	postList, appErr := p.API.GetPostsSince(channelID, (time.Now().Unix()-zoomMeetingTimeWindow)*1000)
	if appErr != nil {
		return false, "", "", "", appErr
	}

	for _, post := range postList.ToSlice() {
		if post.DeleteAt != 0 {
			continue
		}

		meetingProvider := getString("meeting_provider", post.Props)
		if meetingProvider == "" {
			continue
		}

		meetingLink := getString("meeting_link", post.Props)
		if meetingLink == "" {
			continue
		}

		meetingStatus := getString("meeting_status", post.Props)
		if meetingStatus == zoom.WebhookStatusEnded {
			continue
		}

		creator := getString("meeting_creator_username", post.Props)

		return true, meetingLink, creator, meetingProvider, nil
	}

	return false, "", "", "", nil
}

func getString(key string, props model.StringInterface) string {
	value := ""
	if valueInterface, ok := props[key]; ok {
		if valueString, ok := valueInterface.(string); ok {
			value = valueString
		}
	}
	return value
}

func (p *Plugin) deauthorizeUser(w http.ResponseWriter, r *http.Request) {
	// TODO: Check if we need to perform Zoom's webhook verification here https://github.com/mattermost/mattermost-plugin-zoom/issues/291
	if !p.verifyMattermostWebhookSecret(r) {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req zoom.DeauthorizationEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := p.fetchOAuthUserInfo(zoomUserByZoomID, req.Payload.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = p.disconnectOAuthUser(info.UserID); err != nil {
		http.Error(w, "Unable to disconnect user from Zoom", http.StatusInternalServerError)
		return
	}

	message := "We have received a deauthorization message from Zoom for your account. We have removed all your Zoom related information from our systems. Please, connect again to Zoom to keep using it."
	if err = p.sendDirectMessage(info.UserID, message); err != nil {
		p.API.LogWarn("failed to dm user about deauthorization", "error", err.Error())
	}

	if req.Payload.UserDataRetention == falseString {
		if err := p.completeCompliance(req.Payload); err != nil {
			p.API.LogWarn("failed to complete compliance after user deauthorization", "error", err.Error())
		}
	}
}

func (p *Plugin) completeCompliance(payload zoom.DeauthorizationPayload) error {
	data := zoom.ComplianceRequest{
		ClientID:                     payload.ClientID,
		UserID:                       payload.UserID,
		AccountID:                    payload.AccountID,
		DeauthorizationEventReceived: payload,
		ComplianceCompleted:          true,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "could not marshal JSON data")
	}

	res, err := http.Post(
		p.getZoomAPIURL()+"/oauth/data/compliance",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return errors.Wrap(err, "could not make POST request to the data compliance endpoint")
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return errors.Errorf("data compliance request has failed with status code: %d", res.StatusCode)
	}

	return nil
}

// parseOAuthUserState parses the user ID and the channel ID from the given OAuth user state.
func parseOAuthUserState(state string) (userID, channelID string, justConnect bool, err error) {
	stateComponents := strings.Split(state, "_")
	if len(stateComponents) != zoomOAuthUserStateLength {
		return "", "", false, errors.New("invalid OAuth user state")
	}

	return stateComponents[1], stateComponents[2], stateComponents[3] == trueString, nil
}

func (p *Plugin) sendUserSettingForm(userID, channelID, rootID string) error {
	var currentValue string
	userPMISettingPref, err := p.getPMISettingData(userID)
	if err != nil {
		return err
	}

	switch userPMISettingPref {
	case "", zoomPMISettingValueAsk:
		currentValue = ask
	case trueString:
		currentValue = yes
	default:
		currentValue = no
	}

	slackAttachment := p.slackAttachmentToUpdatePMI(currentValue)
	post := &model.Post{
		ChannelId: channelID,
		UserId:    p.botUserID,
		RootId:    rootID,
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{slackAttachment})
	p.API.SendEphemeralPost(userID, post)
	return nil
}

func (p *Plugin) slackAttachmentToUpdatePMI(currentValue string) *model.SlackAttachment {
	apiEndPoint := fmt.Sprintf("/plugins/%s%s", manifest.Id, pathUpdatePMI)

	slackAttachment := model.SlackAttachment{
		Fallback: "Failed to set your preference",
		Title:    "*Setting: Use your Personal Meeting ID*",
		Text:     fmt.Sprintf("\n\nDo you want to use your Personal Meeting ID when starting a meeting?\n\nCurrent value: %s", currentValue),
		Actions: []*model.PostAction{
			{
				Id:    yes,
				Name:  yes,
				Type:  model.PostActionTypeButton,
				Style: "default",
				Integration: &model.PostActionIntegration{
					URL: apiEndPoint,
					Context: map[string]interface{}{
						actionForContext: yes,
					},
				},
			},
			{
				Id:    no,
				Name:  no,
				Type:  model.PostActionTypeButton,
				Style: "default",
				Integration: &model.PostActionIntegration{
					URL: apiEndPoint,
					Context: map[string]interface{}{
						actionForContext: no,
					},
				},
			},
			{
				Id:    ask,
				Name:  ask,
				Type:  model.PostActionTypeButton,
				Style: "default",
				Integration: &model.PostActionIntegration{
					URL: apiEndPoint,
					Context: map[string]interface{}{
						actionForContext: ask,
					},
				},
			},
		},
	}

	return &slackAttachment
}

func (p *Plugin) isChannelRestrictedForMeetings(channelID string) (bool, error) {
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return false, errors.New(appErr.Message)
	}

	zoomChannelSettingsMap, err := p.listZoomChannelSettings()
	if err != nil {
		return false, err
	}

	val, exist := zoomChannelSettingsMap[channelID]
	preference := false
	/*
		Check if Zoom settings for current channel exist.
		Check if creating meeting is disabled in the plugin configuration.
	*/
	if exist {
		if val.Preference == ZoomChannelPreferences[DefaultChannelRestrictionPreference] {
			preference = p.configuration.RestrictMeetingCreation
		} else if val.Preference == ZoomChannelPreferences[RestrictPreference] {
			preference = true
		}
	} else if channel.Type == model.ChannelTypeOpen {
		preference = p.configuration.RestrictMeetingCreation
	}

	return preference, nil
}

func (mv ZoomChannelSettingsMapValue) IsValid() error {
	if mv.Preference == "" {
		return errors.New("preference should not be empty")
	}

	found := false
	for _, value := range ZoomChannelPreferences {
		if value == mv.Preference {
			found = true
			break
		}
	}
	if !found {
		return errors.New("invalid preference")
	}

	return nil
}

func (p *Plugin) handleMeetingCreation(channelID, rootID, topic string, user *model.User, zoomUser *zoom.User) (string, error) {
	var meetingID int
	var createMeetingErr error
	userPMISettingPref, err := p.getPMISettingData(user.Id)
	if err != nil {
		return "", errors.Wrap(err, "error fetching PMI setting data")
	}

	createMeetingWithPMI := false
	switch userPMISettingPref {
	case "", zoomPMISettingValueAsk:
		p.askPreferenceForMeeting(user.Id, channelID, rootID)
		return "", nil
	case trueString:
		createMeetingWithPMI = true
		meetingID = zoomUser.Pmi

		if meetingID <= 0 {
			meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, topic)
			if createMeetingErr != nil {
				return "", createMeetingErr
			}
			p.sendEnableZoomPMISettingMessage(user.Id, channelID, rootID)
		}
	default:
		meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, topic)
		if createMeetingErr != nil {
			return "", createMeetingErr
		}
	}

	if postMeetingErr := p.postMeeting(user, meetingID, channelID, rootID, topic); postMeetingErr != nil {
		return "", createMeetingErr
	}

	p.trackMeetingStart(user.Id, telemetryStartSourceCommand)
	p.trackMeetingType(user.Id, createMeetingWithPMI)

	meetingURL := p.getMeetingURL(user, meetingID)
	return meetingURL, nil
}
