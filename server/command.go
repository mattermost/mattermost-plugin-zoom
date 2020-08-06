package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

const (
	helpText      = `* |/zoom start| - Start a zoom meeting`
	oAuthHelpText = `* |/zoom disconnect| - Disconnect from zoom`
)

func (p *Plugin) getCommand() *model.Command {
	return &model.Command{
		Trigger:          "zoom",
		DisplayName:      "Zoom",
		Description:      "Zoom Integration.",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: start, disconnect, help",
		AutoCompleteHint: "[command]",
		AutocompleteData: p.getAutocompleteData(),
	}
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

	recentMeeting, recentMeetingID, creatorName, appErr := p.checkPreviousMessages(args.ChannelId)
	if appErr != nil {
		return "Error checking previous messages", nil
	}

	if recentMeeting {
		p.postConfirm(recentMeetingID, args.ChannelId, "", userID, creatorName)
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
	text := "###### Mattermost Zoom Plugin - Slash Command Help\n"
	text += strings.Replace(helpText, "|", "`", -1)

	if p.configuration.EnableOAuth {
		text += "\n" + strings.Replace(oAuthHelpText, "|", "`", -1)
	}
	return text, nil
}

// getAutocompleteData retrieves auto-complete data for the "/zoom" command
func (p *Plugin) getAutocompleteData() *model.AutocompleteData {
	zoom := model.NewAutocompleteData("zoom", "[command]", "Available commands: start, disconnect, help")

	start := model.NewAutocompleteData("start", "", "Starts a Zoom meeting")
	zoom.AddCommand(start)

	// no point in showing the 'disconnect' option if OAuth is not enabled
	if p.configuration.EnableOAuth {
		disconnect := model.NewAutocompleteData("disconnect", "", "Disonnects from Zoom")
		zoom.AddCommand(disconnect)
	}

	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
