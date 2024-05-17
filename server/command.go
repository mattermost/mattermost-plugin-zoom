package main

import (
	"fmt"
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
	helpText      = `* |/zoom start| - Start a zoom meeting`
	oAuthHelpText = `* |/zoom connect| - Connect to Zoom
* |/zoom disconnect| - Disconnect from Zoom`
	settingHelpText        = `* |/zoom settings| - Update your preferences`
	alreadyConnectedText   = "Already connected"
	zoomPreferenceCategory = "plugin:zoom"
	zoomPMISettingName     = "use-pmi"
	zoomPMISettingValueAsk = "ask"
)

const (
	actionConnect     = "connect"
	actionSubscribe   = "subscribe"
	actionUnsubscribe = "unsubscribe"
	actionStart       = "start"
	actionDisconnect  = "disconnect"
	actionHelp        = "help"
	settings          = "settings"
)

func (p *Plugin) getCommand() (*model.Command, error) {
	iconData, err := command.GetIconData(p.API, "assets/profile.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	canConnect := !p.configuration.AccountLevelApp

	autoCompleteDesc := "Available commands: start, help, settings"
	if canConnect {
		autoCompleteDesc = "Available commands: start, connect, disconnect, help, settings"
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

func (p *Plugin) parseCommand(rawCommand string) (cmd, action, topic string, meetingID int) {
	split := strings.Fields(rawCommand)
	cmd = split[0]
	if len(split) > 1 {
		action = split[1]
	}
	if action == actionStart {
		topic = strings.Join(split[2:], " ")
	}
	if len(split) > 2 && (action == actionSubscribe || action == actionUnsubscribe) {
		meetingID, _ = strconv.Atoi(split[2])
	}
	return cmd, action, topic, meetingID
}

func (p *Plugin) executeCommand(c *plugin.Context, args *model.CommandArgs) (string, error) {
	command, action, topic, meetingID := p.parseCommand(args.Command)

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
	case actionSubscribe:
		return p.runSubscribeCommand(user, args, meetingID)
	case actionUnsubscribe:
		return p.runUnsubscribeCommand(user, args, meetingID)
	case actionStart:
		return p.runStartCommand(args, user, topic)
	case actionDisconnect:
		return p.runDisconnectCommand(user)
	case actionHelp, "":
		return p.runHelpCommand(user)
	case settings:
		return p.runSettingCommand(args, strings.Fields(args.Command)[2:], user)
	default:
		return fmt.Sprintf("Unknown action %v", action), nil
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
			meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, args.ChannelId, topic)
			if createMeetingErr != nil {
				return "", errors.Wrap(createMeetingErr, "failed to create the meeting")
			}
			p.sendEnableZoomPMISettingMessage(user.Id, args.ChannelId, args.RootId)
		}
	default:
		meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, args.ChannelId, topic)
		if createMeetingErr != nil {
			return "", errors.Wrap(createMeetingErr, "failed to create the meeting")
		}
	}

	meeting, err := p.getMeeting(user, meetingID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the meeting")
	}

	if postMeetingErr := p.postMeeting(user, meetingID, meeting.UUID, args.ChannelId, args.RootId, topic); postMeetingErr != nil {
		return "", postMeetingErr
	}

	p.trackMeetingStart(args.UserId, telemetryStartSourceCommand)
	p.trackMeetingType(args.UserId, createMeetingWithPMI)
	return "", nil
}

func (p *Plugin) runConnectCommand(user *model.User, extra *model.CommandArgs) (string, error) {
	if !p.canConnect(user) {
		return "Unknown action `connect`", nil
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

func (p *Plugin) runSubscribeCommand(user *model.User, extra *model.CommandArgs, meetingID int) (string, error) {
	if !p.API.HasPermissionToChannel(user.Id, extra.ChannelId, model.PermissionCreatePost) {
		return "You do not have permission to subscribe to this channel", nil
	}

	meeting, err := p.getMeeting(user, meetingID)
	if err != nil {
		return "Can not subscribe to meeting: meeting not found", errors.Wrap(err, "meeting not found")
	}

	if meeting.Type == zoom.MeetingTypePersonal {
		return "Can not subscribe to personal meeting", nil
	}

	if appErr := p.storeChannelForMeeting(meetingID, extra.ChannelId); appErr != nil {
		return "", errors.Wrap(appErr, "cannot subscribe to meeting")
	}
	return "Channel subscribed to meeting", nil
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
	return "Channel unsubscribed from meeting", nil
}

// runDisconnectCommand runs command to disconnect from Zoom. Will fail if user cannot connect.
func (p *Plugin) runDisconnectCommand(user *model.User) (string, error) {
	if !p.canConnect(user) {
		return "Unknown action `disconnect`", nil
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
		return "Could not disconnect OAuth from zoom, " + err.Error(), nil
	}

	p.trackDisconnect(user.Id)

	return "User disconnected from Zoom.", nil
}

// runHelpCommand runs command to display help text.
func (p *Plugin) runHelpCommand(user *model.User) (string, error) {
	text := starterText + strings.ReplaceAll(helpText+"\n"+settingHelpText, "|", "`")
	if p.canConnect(user) {
		text += "\n" + strings.ReplaceAll(oAuthHelpText, "|", "`")
	}

	return text, nil
}

func (p *Plugin) runSettingCommand(args *model.CommandArgs, params []string, user *model.User) (string, error) {
	if len(params) == 0 {
		if err := p.sendUserSettingForm(user.Id, args.ChannelId, args.RootId); err != nil {
			return "", err
		}
		return "", nil
	}
	return fmt.Sprintf("Unknown Action %v", ""), nil
}

func (p *Plugin) updateUserPersonalSettings(usePMIValue, userID string) *model.AppError {
	return p.API.UpdatePreferencesForUser(userID, []model.Preference{
		{
			UserId:   userID,
			Category: zoomPreferenceCategory,
			Name:     zoomPMISettingName,
			Value:    usePMIValue,
		},
	})
}

// getAutocompleteData retrieves auto-complete data for the "/zoom" command
func (p *Plugin) getAutocompleteData() *model.AutocompleteData {
	canConnect := !p.configuration.AccountLevelApp

	available := "start, help, settings, subscribe, unsubscribe"
	if canConnect {
		available = "start, connect, disconnect, help, settings, subscribe, unsubscribe"
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

	subscribe := model.NewAutocompleteData("subscribe", "[meeting id]", "Subscribe this channel to a Zoom meeting")
	zoom.AddCommand(subscribe)

	unsubscribe := model.NewAutocompleteData("unsubscribe", "[meeting id]", "Unsubscribe this channel from a Zoom meeting")
	zoom.AddCommand(unsubscribe)

	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
