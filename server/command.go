// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"
	"github.com/pkg/errors"
)

const (
	starterText   = "###### Mattermost Zoom Plugin - Slash Command Help\n"
	helpText      = `* |/zoom start| - Start a Zoom meeting`
	oAuthHelpText = `* |/zoom connect| - Connect to Zoom
* |/zoom disconnect| - Disconnect from Zoom`
	settingHelpText               = `* |/zoom settings| - Update your preferences`
	channelPreferenceHelpText     = `* |/zoom channel-settings| - Update your current channel preference`
	listChannelPreferenceHelpText = `* |/zoom channel-settings list| - List all channel preferences`
	subscriptionHelpText          = `* |/zoom subscription add [meetingID]| - Subscribe this channel to a Zoom meeting
* |/zoom subscription remove [meetingID]| - Unsubscribe this channel from a Zoom meeting
* |/zoom subscription list| - List all meeting subscriptions`
	alreadyConnectedText   = "Already connected"
	zoomPreferenceCategory = "plugin:zoom"
	zoomPMISettingName     = "use-pmi"
	zoomPMISettingValueAsk = "ask"
)

const (
	actionConnect             = "connect"
	actionStart               = "start"
	actionDisconnect          = "disconnect"
	actionHelp                = "help"
	actionSubscription        = "subscription"
	subscriptionActionAdd     = "add"
	subscriptionActionRemove  = "remove"
	subscriptionActionList    = "list"
	settings                  = "settings"
	actionChannelSettings     = "channel-settings"
	channelSettingsActionList = "list"

	actionUnknown = "Unknown Action"
)

const channelPreferenceListErr = "Unable to list channel preferences"

func (p *Plugin) getCommand() (*model.Command, error) {
	iconData, err := command.GetIconData(p.API, "assets/profile.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	canConnect := !p.configuration.AccountLevelApp

	autoCompleteDesc := "Available commands: start, help, subscription, settings, channel-settings"
	if canConnect {
		autoCompleteDesc = "Available commands: start, connect, disconnect, help, subscription, settings, channel-settings"
	}

	return &model.Command{
		Trigger:              "zoom",
		AutoComplete:         true,
		AutoCompleteDesc:     autoCompleteDesc,
		AutoCompleteHint:     "[command]",
		AutocompleteData:     p.getAutocompleteData(),
		AutocompleteIconData: iconData,
	}, nil
}

func (p *Plugin) postCommandResponse(args *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: args.ChannelId,
		Message:   text,
	}
	_ = p.API.SendEphemeralPost(args.UserId, post)
}

func (p *Plugin) parseCommand(rawCommand string) (cmd, action, topic string) {
	split := strings.Fields(rawCommand)
	cmd = split[0]
	if len(split) > 1 {
		action = split[1]
	}
	if action == actionStart {
		topic = strings.Join(split[2:], " ")
	}
	return cmd, action, topic
}

func (p *Plugin) executeCommand(c *plugin.Context, args *model.CommandArgs) (string, error) {
	command, action, topic := p.parseCommand(args.Command)

	if command != "/zoom" {
		return fmt.Sprintf("Command '%s' is not /zoom. Please try again.", command), nil
	}

	if action == "" {
		return "Please specify an action for /zoom command.", nil
	}

	userID := args.UserId
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return fmt.Sprintf("We could not retrieve user (userId: %v)", args.UserId), nil
	}

	switch action {
	case actionConnect:
		return p.runConnectCommand(user, args)
	case actionSubscription:
		return p.runSubscriptionCommand(args, strings.Fields(args.Command)[2:], user)
	case actionStart:
		return p.runStartCommand(args, user, topic)
	case actionDisconnect:
		return p.runDisconnectCommand(user)
	case actionHelp, "":
		return p.runHelpCommand(user)
	case settings:
		return p.runSettingCommand(args, strings.Fields(args.Command)[2:], user)
	case actionChannelSettings:
		return p.runChannelSettingsCommand(args, strings.Fields(args.Command)[2:], user)
	default:
		return fmt.Sprintf("%s %v", actionUnknown, action), nil
	}
}

func (p *Plugin) canConnect(user *model.User) bool {
	return !p.configuration.AccountLevelApp || user.IsSystemAdmin() // admins can connect Account level apps
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	msg, err := p.executeCommand(c, args)
	if err != nil {
		p.API.LogWarn("failed to execute command", "error", err.Error())
	}
	if msg != "" {
		p.postCommandResponse(args, msg)
	}
	return &model.CommandResponse{}, nil
}

// runStartCommand runs command to start a Zoom meeting.
func (p *Plugin) runStartCommand(args *model.CommandArgs, user *model.User, topic string) (string, error) {
	restrict, err := p.isChannelRestrictedForMeetings(args.ChannelId)
	if err != nil {
		p.client.Log.Error("Unable to check channel preference", "ChannelID", args.ChannelId, "Error", err.Error())
		return "Error occurred while starting meeting", nil
	}

	if restrict {
		return "Creating Zoom meeting is disabled for this channel.", nil
	}

	if _, appErr := p.API.GetChannelMember(args.ChannelId, user.Id); appErr != nil {
		return fmt.Sprintf("We could not get the channel members (channelId: %v)", args.ChannelId), nil
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
	if authErr != nil {
		// the user state will be needed later while connecting the user to Zoom via OAuth
		if appErr := p.storeOAuthUserState(user.Id, args.ChannelId, false); appErr != nil {
			p.API.LogWarn("failed to store user state")
		}
		return authErr.Message, authErr.Err
	}

	recentMeeting, recentMeetingLink, creatorName, provider, appErr := p.checkPreviousMessages(args.ChannelId)
	if appErr != nil {
		return "Error checking previous messages", nil
	}

	if recentMeeting {
		p.postConfirm(recentMeetingLink, args.ChannelId, topic, user.Id, args.RootId, creatorName, provider)
		return "", nil
	}

	var meetingID int
	var meetingUUID string
	var createMeetingErr error

	userPMISettingPref, err := p.getPMISettingData(user.Id)
	if err != nil {
		return "", err
	}

	createMeetingWithPMI := false
	switch userPMISettingPref {
	case "", zoomPMISettingValueAsk:
		p.askPreferenceForMeeting(user.Id, args.ChannelId, args.RootId)
		return "", nil
	case trueString:
		createMeetingWithPMI = true
		meetingID = zoomUser.Pmi

		if meetingID <= 0 {
			meetingID, meetingUUID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, topic)
			if createMeetingErr != nil {
				return "", errors.Wrap(createMeetingErr, "failed to create the meeting")
			}
			p.sendEnableZoomPMISettingMessage(user.Id, args.ChannelId, args.RootId)
		}
	default:
		meetingID, meetingUUID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, topic)
		if createMeetingErr != nil {
			return "", errors.Wrap(createMeetingErr, "failed to create the meeting")
		}
	}

	if postMeetingErr := p.postMeeting(user, meetingID, meetingUUID, args.ChannelId, args.RootId, topic); postMeetingErr != nil {
		return "", postMeetingErr
	}

	p.trackMeetingStart(args.UserId, telemetryStartSourceCommand)
	p.trackMeetingType(args.UserId, createMeetingWithPMI)
	return "", nil
}

func (p *Plugin) runConnectCommand(user *model.User, extra *model.CommandArgs) (string, error) {
	if !p.canConnect(user) {
		return fmt.Sprintf("%s `%s`", actionUnknown, actionConnect), nil
	}

	oauthMsg := fmt.Sprintf(
		zoom.OAuthPrompt,
		*p.API.GetConfig().ServiceSettings.SiteURL)

	// OAuth Account Level
	if p.configuration.AccountLevelApp {
		token, err := p.getSuperuserToken()
		if err == nil && token != nil {
			return alreadyConnectedText, nil
		}

		appErr := p.storeOAuthUserState(user.Id, extra.ChannelId, true)
		if appErr != nil {
			return "", errors.Wrap(appErr, "cannot store state")
		}
		return oauthMsg, nil
	}

	// OAuth User Level
	_, err := p.fetchOAuthUserInfo(zoomUserByMMID, user.Id)
	if err == nil {
		return alreadyConnectedText, nil
	}

	appErr := p.storeOAuthUserState(user.Id, extra.ChannelId, true)
	if appErr != nil {
		return "", errors.Wrap(appErr, "cannot store state")
	}
	return oauthMsg, nil
}

func (p *Plugin) runSubscriptionCommand(args *model.CommandArgs, params []string, user *model.User) (string, error) {
	if len(params) == 0 {
		return "Please specify a subscription action: `add`, `remove`, or `list`.\nUsage: `/zoom subscription [action] [meetingID]`", nil
	}

	switch params[0] {
	case subscriptionActionAdd:
		if len(params) < 2 {
			return "Please specify a meeting ID. Usage: `/zoom subscription add [meetingID]`", nil
		}
		meetingID, err := strconv.Atoi(params[1])
		if err != nil {
			return "Invalid meeting ID. Please provide a numeric meeting ID.", nil
		}
		return p.runSubscribeCommand(user, args, meetingID)
	case subscriptionActionRemove:
		if len(params) < 2 {
			return "Please specify a meeting ID. Usage: `/zoom subscription remove [meetingID]`", nil
		}
		meetingID, err := strconv.Atoi(params[1])
		if err != nil {
			return "Invalid meeting ID. Please provide a numeric meeting ID.", nil
		}
		return p.runUnsubscribeCommand(user, args, meetingID)
	case subscriptionActionList:
		return p.runSubscriptionListCommand(args)
	default:
		return fmt.Sprintf("Unknown subscription action: `%s`. Available actions: `add`, `remove`, `list`.", params[0]), nil
	}
}

func (p *Plugin) runSubscriptionListCommand(args *model.CommandArgs) (string, error) {
	if !p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionCreatePost) {
		return "You do not have permission to view subscriptions in this channel.", nil
	}

	subs, err := p.listAllMeetingSubscriptions()
	if err != nil {
		p.client.Log.Error("Unable to list meeting subscriptions", "Error", err.Error())
		return "Unable to list meeting subscriptions.", nil
	}

	if len(subs) == 0 {
		return "No meeting subscriptions found.", nil
	}

	var sb strings.Builder
	sb.WriteString("#### Meeting Subscriptions\n\n")
	sb.WriteString("| Meeting ID | Channel |\n")
	sb.WriteString("| :--- | :--- |\n")

	for meetingID, channelID := range subs {
		channel, appErr := p.client.Channel.Get(channelID)
		if appErr != nil {
			p.client.Log.Error("Unable to get channel for subscription list", "ChannelID", channelID, "Error", appErr.Error())
			continue
		}
		sb.WriteString(fmt.Sprintf("| %s | ~%s |\n", meetingID, channel.Name))
	}

	return sb.String(), nil
}

func (p *Plugin) runSubscribeCommand(user *model.User, extra *model.CommandArgs, meetingID int) (string, error) {
	if !p.API.HasPermissionToChannel(user.Id, extra.ChannelId, model.PermissionCreatePost) {
		return "You do not have permission to subscribe to this channel", nil
	}

	meeting, err := p.getMeeting(user, meetingID)
	if err != nil {
		return "Cannot subscribe to meeting: meeting not found", errors.Wrap(err, "meeting not found")
	}

	if meeting.Type == zoom.MeetingTypePersonal {
		return "Cannot subscribe to personal meeting", nil
	}

	if appErr := p.storeChannelForMeeting(meetingID, extra.ChannelId); appErr != nil {
		return "", errors.Wrap(appErr, "cannot subscribe to meeting")
	}

	return "Channel subscribed to meeting.", nil
}

func (p *Plugin) runUnsubscribeCommand(user *model.User, extra *model.CommandArgs, meetingID int) (string, error) {
	if !p.API.HasPermissionToChannel(user.Id, extra.ChannelId, model.PermissionCreatePost) {
		return "You do not have permission to unsubscribe from this channel", nil
	}

	_, err := p.getMeeting(user, meetingID)
	if err != nil {
		return "Can not unsubscribe from meeting: meeting not accesible in zoom", errors.Wrap(err, "meeting not accesible in zoom")
	}

	if channelID, appErr := p.fetchChannelForMeeting(meetingID); appErr != nil || channelID == "" {
		return "Can not unsubscribe from meeting: meeting not found", errors.New("meeting not found")
	}
	if appErr := p.deleteChannelForMeeting(meetingID); appErr != nil {
		return "Can not unsubscribe from meeting: unable to delete the meeting subscription", errors.Wrap(appErr, "cannot unsubscribe from meeting")
	}

	return "Channel unsubscribed from meeting.", nil
}

// runDisconnectCommand runs command to disconnect from Zoom. Will fail if user cannot connect.
func (p *Plugin) runDisconnectCommand(user *model.User) (string, error) {
	if !p.canConnect(user) {
		return fmt.Sprintf("%s `%s`", actionUnknown, actionDisconnect), nil
	}

	if p.configuration.AccountLevelApp {
		err := p.removeSuperUserToken()
		if err != nil {
			return "Error disconnecting, " + err.Error(), nil
		}
		return "Successfully disconnected from Zoom.", nil
	}

	err := p.disconnectOAuthUser(user.Id)
	if err != nil {
		return "Could not disconnect OAuth from Zoom, " + err.Error(), nil
	}

	p.trackDisconnect(user.Id)

	return "User disconnected from Zoom.", nil
}

// runHelpCommand runs command to display help text.
func (p *Plugin) runHelpCommand(user *model.User) (string, error) {
	text := starterText + strings.ReplaceAll(helpText+"\n"+settingHelpText+"\n"+subscriptionHelpText, "|", "`")
	if p.API.HasPermissionTo(user.Id, model.PermissionManageSystem) {
		text += "\n" + strings.ReplaceAll(channelPreferenceHelpText+"\n"+listChannelPreferenceHelpText, "|", "`")
	}

	if p.canConnect(user) {
		text += "\n" + strings.ReplaceAll(oAuthHelpText, "|", "`")
	}

	return text, nil
}

func (p *Plugin) runSettingCommand(args *model.CommandArgs, params []string, user *model.User) (string, error) {
	if _, authErr := p.authenticateAndFetchZoomUser(user); authErr != nil {
		// the user state will be needed later while connecting the user to Zoom via OAuth
		if appErr := p.storeOAuthUserState(user.Id, args.ChannelId, false); appErr != nil {
			p.API.LogWarn("failed to store user state")
		}
		return authErr.Message, authErr.Err
	}

	if len(params) == 0 {
		if err := p.sendUserSettingForm(user.Id, args.ChannelId, args.RootId); err != nil {
			return "", err
		}
		return "", nil
	}
	return actionUnknown, nil
}

func (p *Plugin) runChannelSettingsCommand(args *model.CommandArgs, params []string, user *model.User) (string, error) {
	if len(params) == 0 {
		return p.runEditChannelSettingsCommand(args, user)
	} else if params[0] == channelSettingsActionList {
		return p.runChannelSettingsListCommand(args)
	}

	return actionUnknown, nil
}

func (p *Plugin) runEditChannelSettingsCommand(args *model.CommandArgs, user *model.User) (string, error) {
	if !p.client.User.HasPermissionTo(args.UserId, model.PermissionManageChannelRoles) {
		return "Unable to execute the command, only channel admins have access to execute this command.", nil
	}

	channel, appErr := p.client.Channel.Get(args.ChannelId)
	if appErr != nil {
		p.client.Log.Error("Unable to get channel", "ChannelID", args.ChannelId, "Error", appErr.Error())
		return "Error occurred while fetching channel information", nil
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return "Preference not allowed to set for DM/GM.", nil
	}

	urlStr := fmt.Sprintf("%s/plugins/%s%s", p.siteURL, url.PathEscape(manifest.Id), pathChannelPreference)

	requestBody := model.OpenDialogRequest{
		TriggerId: args.TriggerId,
		URL:       urlStr,
		Dialog: model.Dialog{
			Title:       "Set Channel Preference",
			SubmitLabel: "Submit",
			CallbackId:  channel.DisplayName,
			Elements: []model.DialogElement{
				{
					DisplayName: fmt.Sprintf("Select your channel preference for ~%s", channel.DisplayName),
					HelpText:    "Disable to restrict creating meetings in this channel.",
					Name:        "preference",
					Type:        "radio",
					Options: []*model.PostActionOptions{
						{
							Text:  "Enable Zoom Meetings in this channel",
							Value: "allow",
						},
						{
							Text:  "Disable Zoom Meetings in this channel",
							Value: "restrict",
						},
						{
							Text:  fmt.Sprintf("Default to plugin-wide settings (%t)", p.getConfiguration().RestrictMeetingCreation),
							Value: "default",
						},
					},
				},
			},
		},
	}

	client, _, err := p.getActiveClient(user)
	if err != nil {
		p.client.Log.Error("Unable to get the client", "Error", err.Error())
		return "Unable to send request to open preference dialog", nil
	}

	if err := client.OpenDialogRequest(&requestBody); err != nil {
		p.client.Log.Error("Failed to fulfill the request to open preference dialog", "Error", err.Error())
		return "Unable to open the dialog for setting preference", nil
	}

	return "", nil
}

func (p *Plugin) runChannelSettingsListCommand(args *model.CommandArgs) (string, error) {
	if !p.client.User.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return "Unable to execute the command, only system admins have access to execute this command.", nil
	}

	zoomChannelSettingsMap, err := p.listZoomChannelSettings()
	if err != nil {
		p.client.Log.Error(channelPreferenceListErr, "Error", err.Error())
		return channelPreferenceListErr, nil
	}

	if len(zoomChannelSettingsMap) == 0 {
		return "No channel preference present", nil
	}

	var sb strings.Builder
	sb.WriteString("#### Channel preferences\n")
	config := p.getConfiguration()
	if config.RestrictMeetingCreation {
		sb.WriteString("Default: Allow meetings only in private channels and DMs/GMs\n\n")
	} else {
		sb.WriteString("Default: Allow meetings in public channels, private channels, and DMs/GMs\n\n")
	}

	listChannelHeading := true
	for key, value := range zoomChannelSettingsMap {
		preference := value.Preference
		channel, err := p.client.Channel.Get(key)
		if err != nil {
			p.client.Log.Error(channelPreferenceListErr, "Error", err.Error())
			return channelPreferenceListErr, nil
		}
		if value.Preference == ZoomChannelPreferences[DefaultChannelRestrictionPreference] {
			continue
		}

		if listChannelHeading {
			sb.WriteString("| Channel ID | Channel Name | Preference |\n| :---- | :-------- | :-------- |")
			listChannelHeading = false
		}

		sb.WriteString(fmt.Sprintf("\n|%s|%s|%s|", key, channel.DisplayName, preference))
	}

	return sb.String(), nil
}

// getAutocompleteData retrieves auto-complete data for the "/zoom" command
func (p *Plugin) getAutocompleteData() *model.AutocompleteData {
	canConnect := !p.configuration.AccountLevelApp

	available := "start, help, subscription, settings, channel-settings"
	if canConnect {
		available = "start, connect, disconnect, help, subscription, settings, channel-settings"
	}

	zoom := model.NewAutocompleteData("zoom", "[command]", fmt.Sprintf("Available commands: %s", available))
	start := model.NewAutocompleteData("start", "[meeting topic]", "Starts a Zoom meeting with a topic (optional)")
	zoom.AddCommand(start)

	// no point in showing the 'disconnect' option if OAuth is not enabled
	if canConnect {
		connect := model.NewAutocompleteData("connect", "", "Connect to Zoom")
		disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect from Zoom")
		zoom.AddCommand(connect)
		zoom.AddCommand(disconnect)
	}

	// setting to allow the user to decide whether to use PMI for instant meetings
	setting := model.NewAutocompleteData("settings", "", "Update your meeting ID preferences")
	zoom.AddCommand(setting)

	subscription := model.NewAutocompleteData("subscription", "[action]", "Manage meeting subscriptions")
	subAdd := model.NewAutocompleteData("add", "[meeting id]", "Subscribe this channel to a Zoom meeting")
	subRemove := model.NewAutocompleteData("remove", "[meeting id]", "Unsubscribe this channel from a Zoom meeting")
	subList := model.NewAutocompleteData("list", "", "List all meeting subscriptions and their channels")
	subscription.AddCommand(subAdd)
	subscription.AddCommand(subRemove)
	subscription.AddCommand(subList)
	zoom.AddCommand(subscription)

	// channel-settings to update channel preferences
	channelSettings := model.NewAutocompleteData("channel-settings", "", "Update current channel preference")
	channelSettingsList := model.NewAutocompleteData("list", "", "List all the channel preferences")
	channelSettings.AddCommand(channelSettingsList)
	channelSettings.RoleID = model.SystemAdminRoleId
	zoom.AddCommand(channelSettings)

	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
