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

const helpText = `* |/zoom start| - Start a zoom meeting`

const oAuthHelpText = `* |/zoom connect| - Connect to zoom
* |/zoom disconnect| - Disconnect from zoom`

const followStatusHelpText = `* $/zoom follow_status [on|off] $ - Automatically update your Mattermost status based on Zoom status"`

const alreadyConnectedString = "Already connected"

func (p *Plugin) getCommand() (*model.Command, error) {
	iconData, err := command.GetIconData(p.API, "assets/profile.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	return &model.Command{
		Trigger:              "zoom",
		AutoComplete:         true,
		AutoCompleteDesc:     "Available commands: start, disconnect, help",
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
	case "follow_status":
		return p.runEnableDisableFollowStatus(user, args)
	case "help", "":
		return p.runHelpCommand(user)
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

	if err := p.postMeeting(user, zoomUser.Pmi, args.ChannelId, ""); err != nil {
		return "Failed to post message. Please try again.", nil
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
	text := "###### Mattermost Zoom Plugin - Slash Command Help\n"
	text += strings.ReplaceAll(helpText, "|", "`")

	if p.canConnect(user) {
		text += "\n" + strings.ReplaceAll(oAuthHelpText, "|", "`")
	}

	text += "\n" + strings.ReplaceAll(followStatusHelpText, "$", "`")

	return text, nil
}

func (p *Plugin) runEnableDisableFollowStatus(user *model.User, args *model.CommandArgs) (string, error) {
	split := strings.Fields(args.Command)

	if len(split) == 2 { // get and show status
		enabled, appErr := p.getFollowStatusForUser(user.Id)
		if appErr == nil {
			var status string
			if enabled {
				status = "on"
			} else {
				status = "off"
			}
			return fmt.Sprintf("Your current `follow_status` setting is `%v`", status), nil
		} else {
			return "Your current `follow_status` setting is not set.", nil
		}
	}
	if len(split) == 3 {
		var err *model.AppError
		var text string
		switch split[2] {
		case "on":
			err = p.setFollowStatusForUser(user.Id, true)
			text = "Automatically following your Zoom status."
		case "off":
			err = p.setFollowStatusForUser(user.Id, false)
			text = "Not following your Zoom status."
		default:
			text = fmt.Sprintf("Invalid value `%v`. Accepted values: `on`, `off`.", split[2])
		}
		if err != nil {
			return "Could not set your `follow_status` settings", err
		}
		return text, nil
	} else {
		return "Incorrect number of arguments. Usage: `/zoom follow_status [on|off]`", nil
	}
}

// getAutocompleteData retrieves auto-complete data for the "/zoom" command
func (p *Plugin) getAutocompleteData() *model.AutocompleteData {
	available := "start, help"
	if p.configuration.EnableOAuth && !p.configuration.AccountLevelApp {
		available = "start, connect, disconnect, follow_status [ on | off ], help"
	}
	zoom := model.NewAutocompleteData("zoom", "[command]", fmt.Sprintf("Available commands: %s", available))

	start := model.NewAutocompleteData("start", "", "Starts a Zoom meeting")
	zoom.AddCommand(start)

	// no point in showing the 'disconnect' option if OAuth is not enabled
	if p.configuration.EnableOAuth && !p.configuration.AccountLevelApp {
		connect := model.NewAutocompleteData("connect", "", "Connect to Zoom")
		disconnect := model.NewAutocompleteData("disconnect", "", "Disconnects from Zoom")
		zoom.AddCommand(connect)
		zoom.AddCommand(disconnect)
	}

	followStatus := model.NewAutocompleteData("follow_status", "[on|off]", "Automatically sets your Mattermost status to `dnd` when in a Zoom meeting")
	followStatus.AddStaticListArgument("value", true, []model.AutocompleteListItem{
		{HelpText: "Follow Zoom status automatically", Item: "on"},
		{HelpText: "Do not follow Zoom status", Item: "off"},
	})
	zoom.AddCommand(followStatus)

	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
