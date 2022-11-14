package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"

	"github.com/mattermost/mattermost-plugin-api/experimental/command"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
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
	actionConnect    = "connect"
	actionStart      = "start"
	actionDisconnect = "disconnect"
	actionHelp       = "help"
	settings         = "settings"
)

func (p *Plugin) getCommand() (*model.Command, error) {
	iconData, err := command.GetIconData(p.API, "assets/profile.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	canConnect := p.configuration.EnableOAuth && !p.configuration.AccountLevelApp

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
	case actionStart:
		return p.runStartCommand(args, user, topic)
	case actionDisconnect:
		return p.runDisconnectCommand(user)
	case actionHelp, "":
		return p.runHelpCommand()
	case settings:
		return p.runSettingCommand(args, strings.Fields(args.Command)[2:], user)
	default:
		return fmt.Sprintf("Unknown action %v", action), nil
	}
}

func (p *Plugin) canConnect(user *model.User) bool {
	return p.OAuthEnabled() && // we are not on JWT
		(!p.configuration.AccountLevelApp || // we are on user managed app
			user.IsSystemAdmin()) // admins can connect Account level apps
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	msg, err := p.executeCommand(c, args)
	if err != nil {
		p.API.LogWarn("failed to execute command", "Error", err.Error())
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

	recentMeeting, recentMeetingLink, creatorName, provider, appErr := p.checkPreviousMessages(args.ChannelId)
	if appErr != nil {
		return "Error checking previous messages", nil
	}

	if recentMeeting {
		p.postConfirm(recentMeetingLink, args.ChannelId, topic, user.Id, args.RootId, creatorName, provider)
		return "", nil
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
	if authErr != nil {
		// the user state will be needed later while connecting the user to Zoom via OAuth
		if appErr := p.storeOAuthUserState(user.Id, args.ChannelId, false); appErr != nil {
			p.API.LogWarn("failed to store user state")
		}
		return authErr.Message, authErr.Err
	}
	var meetingID int
	var createMeetingErr error

	userPMISettingPref, getUserPMISettingErr := p.getPMISettingData(user.Id)
	if getUserPMISettingErr != nil {
		p.askUserForMeetingPreference(user.Id, args.ChannelId)
		return "", nil
	}

	switch userPMISettingPref {
	case zoomPMISettingValueAsk:
		p.askUserForMeetingPreference(user.Id, args.ChannelId)
		return "", nil
	case "", trueString:
		meetingID = zoomUser.Pmi
	default:
		meetingID, createMeetingErr = p.createMeetingWithoutPMI(user, zoomUser, args.ChannelId, DefaultMeetingTopic)
		if createMeetingErr != nil {
			return "", errors.New("error while creating a new meeting")
		}
	}

	if postMeetingErr := p.postMeeting(user, meetingID, args.ChannelId, args.RootId, DefaultMeetingTopic); postMeetingErr != nil {
		return "", postMeetingErr
	}

	p.trackMeetingStart(args.UserId, telemetryStartSourceCommand)

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
func (p *Plugin) runHelpCommand() (string, error) {
	text := starterText + strings.ReplaceAll(helpText+"\n"+settingHelpText, "|", "`")
	if p.configuration.EnableOAuth {
		text += "\n" + strings.ReplaceAll(oAuthHelpText, "|", "`")
	}

	return text, nil
}

func (p *Plugin) runSettingCommand(args *model.CommandArgs, params []string, user *model.User) (string, error) {
	settingAction := ""
	if len(params) > 0 {
		settingAction = params[0]
	}
	switch settingAction {
	case "":
		p.updatePMI(user.Id, args.ChannelId)
		return "", nil
	default:
		return fmt.Sprintf("Unknown Action %v", settingAction), nil
	}
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
	canConnect := p.OAuthEnabled() && !p.configuration.AccountLevelApp

	available := "start, help, settings"
	if p.configuration.EnableOAuth && !p.configuration.AccountLevelApp {
		available = "start, connect, disconnect, help, settings"
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
	setting := model.NewAutocompleteData("settings", "[command]", "Update your preferences")
	zoom.AddCommand(setting)

	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
