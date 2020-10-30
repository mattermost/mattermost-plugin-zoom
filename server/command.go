package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-api/experimental/command"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

const (
	starterText        = "###### Mattermost Zoom Plugin - Slash Command Help\n"
	helpText           = `* |/zoom start| - Start a zoom meeting`
	oAuthHelpText      = `* |/zoom disconnect| - Disconnect from zoom`
	settingHelpText    = `* |/zoom setting| - Configurate setting options`
	settingPMIHelpText = `* |/zoom setting usePMI [true|false|ask]| - 
		enable | disable | undecide to use PMI to create meeting
	`
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
	case "start":
		return p.runStartCommand(args, user, userID)
	case "disconnect":
		return p.runDisconnectCommand(userID)
	case "help", "":
		return p.runHelpCommand()
	case "setting":
		return p.runSettingCommand(split[2:])
	default:
		return fmt.Sprintf("Unknown action %v", action), nil
	}
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
func (p *Plugin) runStartCommand(args *model.CommandArgs, user *model.User, userID string) (string, error) {
	if _, appErr := p.API.GetChannelMember(args.ChannelId, userID); appErr != nil {
		return fmt.Sprintf("We could not get channel members (channelId: %v)", args.ChannelId), nil
	}

	recentMeeting, recentMeetingLink, creatorName, provider, appErr := p.checkPreviousMessages(args.ChannelId)
	if appErr != nil {
		return "Error checking previous messages", nil
	}

	if recentMeeting {
		p.postConfirm(recentMeetingLink, args.ChannelId, "", userID, creatorName, provider)
		return "", nil
	}

	zoomUser, authErr := p.authenticateAndFetchZoomUser(user)
	if authErr != nil {
		// the user state will be needed later while connecting the user to Zoom via OAuth
		if appErr := p.storeOAuthUserState(userID, args.ChannelId); appErr != nil {
			p.API.LogWarn("failed to store user state")
		}
		return authErr.Message, authErr.Err
	}

	if err := p.postMeeting(user, zoomUser.Pmi, args.ChannelId, ""); err != nil {
		return "Failed to post message. Please try again.", nil
	}
	return "", nil
}

// runDisconnectCommand runs command to disconnect from Zoom. Will fail if OAuth is not enabled.
func (p *Plugin) runDisconnectCommand(userID string) (string, error) {
	if !p.configuration.EnableOAuth {
		return "Invalid attempt to disconnect; OAuth is not enabled.", nil
	}

	if err := p.disconnectOAuthUser(userID); err != nil {
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
func (p *Plugin) runSettingCommand(settingCommands []string) (string, error) {
	settingAction := ""
	if len(settingCommands) > 0 {
		settingAction = settingCommands[0]
	}
	switch settingAction {
	case "usePMI":
		//here process the usePMI command
		if len(settingCommands) > 1 {
			return p.runPMISettingCommand(settingCommands[1])
		}
		return "Set PMI option to \"true\"|\"false\"|\"ask\"", nil
	case "":
		return strings.ReplaceAll(starterText+settingPMIHelpText, "|", "`"), nil
	default:
		return fmt.Sprintf("Unknown Action %v", settingAction), nil
	}
}

func (p *Plugin) runPMISettingCommand(PMISetting string) (string, error) {
	switch PMISetting {
	case "true", "false", "ask":
		return "", p.API.SavePluginConfig(map[string]interface{}{
			"use-pmi": PMISetting,
		})
	default:
		return fmt.Sprintf("Unknown setting option %v", PMISetting), nil
	}
}

// getAutocompleteData retrieves auto-complete data for the "/zoom" command
func (p *Plugin) getAutocompleteData() *model.AutocompleteData {
	zoom := model.NewAutocompleteData("zoom", "[command]", "Available commands: start, disconnect, help, setting")

	start := model.NewAutocompleteData("start", "", "Starts a Zoom meeting")
	zoom.AddCommand(start)

	// no point in showing the 'disconnect' option if OAuth is not enabled
	if p.configuration.EnableOAuth {
		disconnect := model.NewAutocompleteData("disconnect", "", "Disonnects from Zoom")
		zoom.AddCommand(disconnect)
	}
	//setting
	setting := model.NewAutocompleteData("setting", "[command]", "Configurates options")
	zoom.AddCommand(setting)
	//help
	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
