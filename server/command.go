package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"

	"github.com/mattermost/mattermost-plugin-api/experimental/command"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

const (
	starterText        = "###### Mattermost Zoom Plugin - Slash Command Help\n"
	helpText           = `* |/zoom start| - Start a zoom meeting`
	oAuthHelpText      = `* |/zoom disconnect| - Disconnect from zoom`
	settingHelpText    = `* |/zoom setting| - Configure setting options`
	settingPMIHelpText = `* |/zoom setting usePMI [true/false/ask]| - 
		enable / disable / undecide to use PMI to create meeting
	`
	alreadyConnectedString = "Already connected"
	zoomPreferenceCategory = "plugin:zoom"
	zoomPMISettingName     = "use-pmi"
	zoomPMISettingValueAsk = "ask"
)

func (p *Plugin) getCommand() (*model.Command, error) {
	iconData, err := command.GetIconData(p.API, "assets/profile.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	return &model.Command{
		Trigger:              "zoom",
		AutoComplete:         true,
		AutoCompleteDesc:     "Available commands: start, disconnect, help, setting",
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

func (p *Plugin) executeCommand(c *plugin.Context, args *model.CommandArgs) (string, error) {
	split := strings.Fields(args.Command)
	command := split[0]
	action := ""

	if command != "/zoom" {
		return fmt.Sprintf("Command '%s' is not /zoom. Please try again.", command), nil
	}

	if len(split) > 1 {
		action = split[1]
	} else {
		return "Please specify an action for /zoom command.", nil
	}

	userID := args.UserId
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return fmt.Sprintf("We could not retrieve user (userId: %v)", args.UserId), nil
	}

	switch action {
	case "connect":
		return p.runConnectCommand(user, args)
	case "start":
		return p.runStartCommand(args, user)
	case "disconnect":
		return p.runDisconnectCommand(user)
	case "help", "":
		return p.runHelpCommand()
	case "setting":
		return p.runSettingCommand(split[2:], user)
	default:
		return fmt.Sprintf("Unknown action %v", action), nil
	}
}

func (p *Plugin) canConnect(user *model.User) bool {
	return p.configuration.EnableOAuth && // we are not on JWT
		(!p.configuration.AccountLevelApp || // we are on user managed app
			user.IsSystemAdmin()) // admins can connect Account level apps
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
func (p *Plugin) runStartCommand(args *model.CommandArgs, user *model.User) (string, error) {
	if _, appErr := p.API.GetChannelMember(args.ChannelId, user.Id); appErr != nil {
		return fmt.Sprintf("We could not get channel members (channelId: %v)", args.ChannelId), nil
	}

	recentMeeting, recentMeetingLink, creatorName, provider, appErr := p.checkPreviousMessages(args.ChannelId)
	if appErr != nil {
		return "Error checking previous messages", nil
	}

	if recentMeeting {
		p.postConfirm(recentMeetingLink, args.ChannelId, "", user.Id, creatorName, provider)
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

	if userPMISettingPref, getUserPMISettingErr := p.getPMISettingData(user.Id); getUserPMISettingErr == nil {
		switch userPMISettingPref {
		case "", zoomPMISettingValueAsk:
			p.askUserPMIMeeting(user.Id, args.ChannelId)
		case trueString:
			if err := p.postMeeting(user, zoomUser.Pmi, args.ChannelId, defaultMeetingTopic); err == nil {
				return "", err
			}
		default:
			client, _, err := p.getActiveClient(user)
			if err != nil {
				p.API.LogWarn("Error creating the client", "err", err)
				return "Error creating the client.", nil
			}

			meeting, err := client.CreateMeeting(zoomUser, defaultMeetingTopic)
			if err != nil {
				p.API.LogWarn("Error creating the meeting", "err", err)
				return "Error creating the meeting.", nil
			}
			if err := p.postMeeting(user, meeting.ID, args.ChannelId, ""); err != nil {
				return "Failed to post message. Please try again.", nil
			}
		}
	} else {
		p.askUserPMIMeeting(user.Id, args.ChannelId)
	}
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
			return alreadyConnectedString, nil
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
		return alreadyConnectedString, nil
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
			return "Could not disconnect, err=" + err.Error(), nil
		}
		return "Successfully disconnected from Zoom.", nil
	}

	if err := p.disconnectOAuthUser(user.Id); err != nil {
		return fmt.Sprintf("Failed to disconnect the user: %s", err.Error()), nil
	}
	return "User disconnected from Zoom.", nil
}

// runHelpCommand runs command to display help text.
func (p *Plugin) runHelpCommand() (string, error) {
	text := starterText
	text += strings.ReplaceAll(helpText+settingHelpText, "|", "`")
	if p.configuration.EnableOAuth {
		text += "\n" + strings.ReplaceAll(oAuthHelpText, "|", "`")
	}

	return text, nil
}

// run "/zoom setting" command, e.g: /zoom setting usePMI true
func (p *Plugin) runSettingCommand(settingCommands []string, user *model.User) (string, error) {
	settingAction := ""
	if len(settingCommands) > 0 {
		settingAction = settingCommands[0]
	}
	switch settingAction {
	case "usePMI":
		// here process the usePMI command
		if len(settingCommands) > 1 {
			return p.runPMISettingCommand(settingCommands[1], user)
		}
		return "Set PMI option to \"true\"|\"false\"|\"ask\"", nil
	case "":
		return strings.ReplaceAll(starterText+settingPMIHelpText, "|", "`"), nil
	default:
		return fmt.Sprintf("Unknown Action %v", settingAction), nil
	}
}

func (p *Plugin) runPMISettingCommand(usePMIValue string, user *model.User) (string, error) {
	switch usePMIValue {
	case trueString, falseString, zoomPMISettingValueAsk:
		if appError := p.API.UpdatePreferencesForUser(user.Id, []model.Preference{
			{
				UserId:   user.Id,
				Category: zoomPreferenceCategory,
				Name:     zoomPMISettingName,
				Value:    usePMIValue,
			},
		}); appError != nil {
			return "Cannot update preference in zoom setting", nil
		}
		return fmt.Sprintf("Update successfully, usePMI: %v", usePMIValue), nil
	default:
		return fmt.Sprintf("Unknown setting option %v", usePMIValue), nil
	}
}

// getAutocompleteData retrieves auto-complete data for the "/zoom" command
func (p *Plugin) getAutocompleteData() *model.AutocompleteData {
	available := "start, help, setting"
	if p.configuration.EnableOAuth && !p.configuration.AccountLevelApp {
		available = "start, connect, disconnect, help, setting"
	}
	zoom := model.NewAutocompleteData("zoom", "[command]", fmt.Sprintf("Available commands: %s", available))

	start := model.NewAutocompleteData("start", "", "Starts a Zoom meeting")
	zoom.AddCommand(start)

	// no point in showing the 'disconnect' option if OAuth is not enabled
	if p.configuration.EnableOAuth && !p.configuration.AccountLevelApp {
		connect := model.NewAutocompleteData("connect", "", "Connect to Zoom")
		disconnect := model.NewAutocompleteData("disconnect", "", "Disonnects from Zoom")
		zoom.AddCommand(connect)
		zoom.AddCommand(disconnect)
	}
	// setting
	setting := model.NewAutocompleteData("setting", "[command]", "Configurates options")
	zoom.AddCommand(setting)
	// help
	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
