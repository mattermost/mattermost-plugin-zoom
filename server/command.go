package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

const helpText = `* |/zoom start| - Start a zoom meeting`
const oAuthHelpText = `* |/zoom disconnect| - Disconnect from zoom`

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "zoom",
		DisplayName:      "Zoom",
		Description:      "Integration with Zoom.",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: start, help",
		AutoCompleteHint: "[command]",
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
		if _, appErr = p.API.GetChannelMember(args.ChannelId, userID); appErr != nil {
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

		zoomUser, authErr := p.authenticateAndFetchZoomUser(userID, user.Email, args.ChannelId)
		if authErr != nil {
			return authErr.Message, authErr.Err
		}

		meetingID := zoomUser.Pmi

		err := p.postMeeting(user, meetingID, args.ChannelId, "")
		if err != nil {
			return "Failed to post message. Please try again.", nil
		}
		return "", nil
	case "disconnect":
		if p.configuration.EnableLegacyAuth {
			return fmt.Sprintf("Unknown action %v", action), nil
		}
		err := p.disconnect(userID)
		if err != nil {
			return "Failed to disconnect the user, err=" + err.Error(), nil
		}
		return "User disconnected from Zoom.", nil
	case "help", "":
		text := "###### Mattermost Zoom Plugin - Slash Command Help\n" + strings.Replace(helpText, "|", "`", -1)
		if p.configuration.EnableOAuth {
			text += "\n" + strings.Replace(oAuthHelpText, "|", "`", -1)
		}
		return text, nil
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
