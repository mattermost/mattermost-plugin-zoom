// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	defaultMeetingTopic      = "Zoom Meeting"
	zoomOAuthUserStateLength = 4
	settingDataError         = "something went wrong while getting settings data"
	askForPMIMeeting         = "Would you like to use your personal meeting ID?"
	apiToUpdatePMI           = "/plugins/%s/api/v1/updatePMI"
	apiToAskForPMI           = "/plugins/%s/api/v1/askPMI"
	yes                      = "Yes"
	no                       = "No"
	ask                      = "Ask"
	actionForContext         = "action"
	userIDForContext         = "userID"
	channelIDForContext      = "channelID"
	usePersonalMeetingID     = "USE PERSONAL MEETING ID"
	useAUniqueMeetingID      = "USE A UNIQUE MEETING ID"
	MattermostUserIDHeader   = "Mattermost-User-ID"
)

type startMeetingRequest struct {
	ChannelID string `json:"channel_id"`
	Personal  bool   `json:"personal"`
	Topic     string `json:"topic"`
	MeetingID int    `json:"meeting_id"`
	UsePMI    string `json:"use_pmi"`
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	config := p.getConfiguration()
	if err := config.IsValid(p.isCloudLicense()); err != nil {
		http.Error(w, "This plugin is not configured.", http.StatusNotImplemented)
		return
	}

	switch path := r.URL.Path; path {
	case "/webhook":
		p.handleWebhook(w, r)
	case "/api/v1/meetings":
		p.handleStartMeeting(w, r)
	case "/oauth2/connect":
		p.connectUserToZoom(w, r)
	case "/oauth2/complete":
		p.completeUserOAuthToZoom(w, r)
	case "/deauthorization":
		p.deauthorizeUser(w, r)
	case "/api/v1/updatePMI":
		p.setPMI(w, r)
	case "/api/v1/askPMI":
		p.askPMI(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) askPMI(w http.ResponseWriter, r *http.Request) {
	response := &model.PostActionIntegrationResponse{}
	decoder := json.NewDecoder(r.Body)
	postActionIntegrationRequest := &model.PostActionIntegrationRequest{}
	if err := decoder.Decode(&postActionIntegrationRequest); err != nil {
		p.API.LogError("Error decoding PostActionIntegrationRequest params.", "Error", err.Error())
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to write the response", http.StatusInternalServerError)
		}
		return
	}

	action := postActionIntegrationRequest.Context[actionForContext].(string)
	userID := postActionIntegrationRequest.Context[userIDForContext].(string)
	channelID := postActionIntegrationRequest.Context[channelIDForContext].(string)

	slackAttachment := model.SlackAttachment{
		Text: fmt.Sprintf("You have selected `%s` to start the meeting.", action),
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
		http.Error(w, "failed to write the response", http.StatusInternalServerError)
	}

	p.startMeeting(action, userID, channelID)
}

func (p *Plugin) startMeeting(action string, userID string, channelID string) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		p.API.LogWarn("failed to get the user from userID", "Error", appErr.Error())
		return
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
	if authErr != nil {
		p.API.LogWarn("failed to authenticate and fetch the zoom user", "Error", appErr.Error())
		return
	}

	var meetingID int
	var createMeetingErr error
	if action == usePersonalMeetingID {
		meetingID = zoomUser.Pmi
	} else {
		meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, channelID, defaultMeetingTopic)
	}
	if createMeetingErr != nil {
		p.API.LogWarn("failed to create the meeting", "Error", createMeetingErr.Error())
		return
	}

	if postMeetingErr := p.postMeeting(user, meetingID, channelID, "", defaultMeetingTopic); postMeetingErr != nil {
		p.API.LogWarn("failed to post the meeting", "Error", postMeetingErr.Error())
		return
	}

	p.trackMeetingStart(userID, telemetryStartSourceCommand)
}

func (p *Plugin) setPMI(w http.ResponseWriter, r *http.Request) {
	response := &model.PostActionIntegrationResponse{}
	decoder := json.NewDecoder(r.Body)
	postActionIntegrationRequest := &model.PostActionIntegrationRequest{}
	if err := decoder.Decode(&postActionIntegrationRequest); err != nil {
		p.API.LogError("Error decoding PostActionIntegrationRequest params.", "Error", err.Error())
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
	slackAttachment := p.slackAttachmentToUpdatePMI(action, channel.Id)

	val := ""
	switch action {
	case ask:
		val = zoomPMISettingValueAsk
	case no:
		val = falseString
	default:
		val = trueString
	}

	if err = p.updateUserPersonalSettings(val, mattermostUserID); err != nil {
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
		http.Error(w, "failed to write the response", http.StatusInternalServerError)
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
		p.API.LogWarn("failed to delete OAuth user state from KV store", "Error", appErr.Error())
	}

	conf := p.getOAuthConfig()
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		p.API.LogWarn("failed to create the access token", "Error", err.Error())
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

		p.API.LogWarn("failed to get the user", "Error", authErr.Error())
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
			p.API.LogWarn(msg, "Error", err.Error())
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
	}

	p.trackConnect(userID)

	if justConnect {
		p.postEphemeral(userID, channelID, "Successfully connected to Zoom \nType `/zoom settings` to change your meeting ID preference")
	} else {
		meeting, err := client.CreateMeeting(zoomUser, defaultMeetingTopic)
		if err != nil {
			p.API.LogWarn("Error creating the meeting", "Error", err.Error())
			return
		}

		meetingID := meeting.ID
		if err = p.postMeeting(user, meetingID, channelID, "", ""); err != nil {
			p.API.LogWarn("Failed to post the meeting", "Error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
		p.API.LogWarn("failed to write the response", "Error", err.Error())
	}
}

func (p *Plugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if !p.verifyWebhookSecret(r) {
		p.API.LogWarn("Could not verify webhook secreet")
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		res := fmt.Sprintf("Expected Content-Type 'application/json' for webhook request, received '%s'.", r.Header.Get("Content-Type"))
		p.API.LogWarn(res)
		http.Error(w, res, http.StatusBadRequest)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		p.API.LogWarn("Cannot read the body from Webhook")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var webhook zoom.Webhook
	if err = json.Unmarshal(b, &webhook); err != nil {
		p.API.LogError("Error unmarshaling webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if webhook.Event != zoom.EventTypeMeetingEnded {
		w.WriteHeader(http.StatusOK)
		return
	}

	var meetingWebhook zoom.MeetingWebhook
	if err = json.Unmarshal(b, &meetingWebhook); err != nil {
		p.API.LogError("Error unmarshaling meeting webhook", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.handleMeetingEnded(w, r, &meetingWebhook)
}

func (p *Plugin) handleMeetingEnded(w http.ResponseWriter, r *http.Request, webhook *zoom.MeetingWebhook) {
	meetingPostID := webhook.Payload.Object.ID
	postID, appErr := p.fetchMeetingPostID(meetingPostID)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	post, appErr := p.API.GetPost(postID)
	if appErr != nil {
		p.API.LogWarn("Could not get the meeting post by id", "Error", appErr.Error())
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	start := time.Unix(0, post.CreateAt*int64(time.Millisecond))
	length := int(math.Ceil(float64((model.GetMillis()-post.CreateAt)/1000) / 60))
	startText := start.Format("Mon Jan 2 15:04:05 -0700 MST 2006")
	topic, ok := post.Props["meeting_topic"].(string)
	if !ok {
		topic = defaultMeetingTopic
	}

	meetingID, ok := post.Props["meeting_id"].(float64)
	if !ok {
		meetingID = 0
	}

	slackAttachment := model.SlackAttachment{
		Fallback: fmt.Sprintf("Meeting %s has ended: started at %s, length: %d minute(s).", post.Props["meeting_id"], startText, length),
		Title:    topic,
		Text: fmt.Sprintf(
			"Meeting ID: %d\n\n##### Meeting Summary\n\nDate: %s\n\nMeeting Length: %d minute(s)",
			int(meetingID),
			startText,
			length,
		),
	}

	post.Message = "The meeting has ended."
	post.Props["meeting_status"] = zoom.WebhookStatusEnded
	post.Props["attachments"] = []*model.SlackAttachment{&slackAttachment}

	_, appErr = p.API.UpdatePost(post)
	if appErr != nil {
		p.API.LogWarn("Could not update the post", "Error", appErr.Error())
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	if appErr = p.deleteMeetingPostID(meetingPostID); appErr != nil {
		p.API.LogWarn("failed to delete db entry", "Error", appErr.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		http.Error(w, "failed to write the response", http.StatusInternalServerError)
	}
}

func (p *Plugin) slackAttachmentToUpdatePMI(currentValue, channelID string) *model.SlackAttachment {
	apiEndPoint := fmt.Sprintf(apiToUpdatePMI, manifest.ID)

	slackAttachment := model.SlackAttachment{
		Fallback: "You can not set your preference",
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

func (p *Plugin) sendUserSettingForm(userID string, channelID string) {
	var currentValue string
	if userPMISettingPref, err := p.getPMISettingData(userID); err != nil {
		currentValue = ask
	} else {
		switch userPMISettingPref {
		case zoomPMISettingValueAsk:
			currentValue = ask
		case "", trueString:
			currentValue = yes
		default:
			currentValue = no
		}
	}

	slackAttachment := p.slackAttachmentToUpdatePMI(currentValue, channelID)
	post := &model.Post{
		ChannelId: channelID,
		UserId:    p.botUserID,
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{slackAttachment})
	p.API.SendEphemeralPost(userID, post)
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
		p.API.LogDebug("failed to store post id", "err", appErr)
	}
	return nil
}

func (p *Plugin) askPreferenceForMeeting(userID, channelID string) {
	apiEndPoint := fmt.Sprintf(apiToAskForPMI, manifest.ID)

	slackAttachment := model.SlackAttachment{
		Pretext: askForPMIMeeting,
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
					},
				},
			},
		},
	}

	post := &model.Post{
		ChannelId: channelID,
		UserId:    p.botUserID,
	}
	model.ParseSlackAttachment(post, []*model.SlackAttachment{&slackAttachment})
	p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) getPMISettingData(userID string) (string, error) {
	preferences, reqErr := p.API.GetPreferencesForUser(userID)
	if reqErr != nil {
		return "", errors.New(settingDataError)
	}

	for _, preference := range preferences {
		if preference.UserId == userID && preference.Category == zoomPreferenceCategory && preference.Name == zoomPMISettingName {
			return preference.Value, nil
		}
	}
	return "", nil
}

func (p *Plugin) handleStartMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req startMeetingRequest
	var err error
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	if r.URL.Query().Get("force") == "" {
		recentMeeting, recentMeetingLink, creatorName, provider, cpmErr := p.checkPreviousMessages(req.ChannelID)
		if cpmErr != nil {
			http.Error(w, cpmErr.Error(), cpmErr.StatusCode)
			return
		}

		if recentMeeting {
			_, err = w.Write([]byte(`{"meeting_url": ""}`))
			if err != nil {
				p.API.LogWarn("failed to write the response", "Error", err.Error())
			}
			p.postConfirm(recentMeetingLink, req.ChannelID, req.Topic, userID, "", creatorName, provider)
			return
		}
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
	if authErr != nil {
		_, err = w.Write([]byte(`{"meeting_url": ""}`))
		if err != nil {
			p.API.LogWarn("failed to write the response", "Error", err.Error())
		}

		// the user state will be needed later while connecting the user to Zoom via OAuth
		if appErr := p.storeOAuthUserState(userID, req.ChannelID, false); appErr != nil {
			p.API.LogWarn("failed to store user state")
		}

		p.postAuthenticationMessage(req.ChannelID, userID, authErr.Message)
		return
	}

	topic := req.Topic
	if topic == "" {
		topic = defaultMeetingTopic
	}

	var meetingID int
	var createMeetingErr error
	userPMISettingPref, err := p.getPMISettingData(user.Id)
	if err != nil {
		p.askPreferenceForMeeting(user.Id, req.ChannelID)
		return
	}

	switch userPMISettingPref {
	case zoomPMISettingValueAsk:
		p.askPreferenceForMeeting(user.Id, req.ChannelID)
		return
	case "", trueString:
		meetingID = zoomUser.Pmi
	default:
		meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, req.ChannelID, topic)
		if createMeetingErr != nil {
			p.API.LogWarn("failed to create the meeting", "Error", createMeetingErr.Error())
			return
		}
	}

	if meetingID != -1 {
		if err = p.postMeeting(user, meetingID, req.ChannelID, "", topic); err == nil {
			p.trackMeetingStart(userID, telemetryStartSourceWebapp)
			if r.URL.Query().Get("force") != "" {
				p.trackMeetingForced(userID)
			}

			meetingURL := p.getMeetingURL(user, meetingID)
			if _, err = w.Write([]byte(fmt.Sprintf(`{"meeting_url": "%s"}`, meetingURL))); err != nil {
				p.API.LogWarn("failed to write the response", "Error", err.Error())
			}
			return
		}
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (p *Plugin) createMeetingWithoutPMI(user *model.User, zoomUser *zoom.User, channelID, topic string) (int, error) {
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
		p.API.LogWarn("could not get the active zoom client", "Error", err.Error())
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

func (p *Plugin) postEphemeral(userID, channelID, message string) *model.Post {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
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
		meetingProvider := getString("meeting_provider", post.Props)
		if meetingProvider == "" {
			continue
		}

		meetingLink := getString("meeting_link", post.Props)
		if meetingLink == "" {
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
	if !p.verifyWebhookSecret(r) {
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
		p.API.LogWarn("failed to dm user about deauthorization", "Error", err.Error())
	}

	if req.Payload.UserDataRetention == falseString {
		if err := p.completeCompliance(req.Payload); err != nil {
			p.API.LogWarn("failed to complete compliance after user deauthorization", "Error", err.Error())
		}
	}
}

func (p *Plugin) verifyWebhookSecret(r *http.Request) bool {
	config := p.getConfiguration()
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(config.WebhookSecret)) == 1
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
