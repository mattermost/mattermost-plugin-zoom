package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"

	"github.com/mattermost/mattermost-plugin-api/experimental/command"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const helpText = `* |/zoom start| - Start a zoom meeting`
const adminHelpText = `* |/zoom setup| - Set up the Zoom plugin (System admin only)`

const oAuthHelpText = `* |/zoom connect| - Connect to zoom
* |/zoom disconnect| - Disconnect from zoom`

const alreadyConnectedString = "Already connected"

const (
	actionConnect    = "connect"
	actionStart      = "start"
	actionDisconnect = "disconnect"
	actionHelp       = "help"
)

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
	case "setup":
		message := p.runSetup(c, args)
		return message, nil
	case actionConnect:
		return p.runConnectCommand(user, args)
	case actionStart:
		return p.runStartCommand(args, user, topic)
	case actionDisconnect:
		return p.runDisconnectCommand(user)
	case actionHelp, "":
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

// runSetup starts the plugin setup wizard
func (p *Plugin) runSetup(c *plugin.Context, args *model.CommandArgs) string {
	userID := args.UserId

	isSysAdmin, err := p.isAuthorizedSysAdmin(userID)
	if err != nil {
		p.API.LogWarn("Failed to check if user is System Admin", "err", err.Error())

		return "Error checking user's permissions"
	}

	if !isSysAdmin {
		return "Only System Admins are allowed to setup the plugin."
	}

	err = p.flowManager.StartConfigurationWizard(userID)
	if err != nil {
		return errors.Wrap(err, "Failed to start configuration wizard").Error()
	}

	return ""
}

// runStartCommand runs command to start a Zoom meeting.
func (p *Plugin) runStartCommand(args *model.CommandArgs, user *model.User, topic string) (string, error) {
	if _, appErr := p.API.GetChannelMember(args.ChannelId, user.Id); appErr != nil {
		return fmt.Sprintf("We could not get channel members (channelId: %v)", args.ChannelId), nil
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

	client, _, err := p.getActiveClient(user)
	if err != nil {
		p.API.LogWarn("Error creating the client", "err", err)
		return "Error creating the client.", nil
	}

	meeting, err := client.CreateMeeting(zoomUser, topic)
	if err != nil {
		p.API.LogWarn("Error creating the meeting", "err", err)
		return "Error creating the meeting.", nil
	}

	if err := p.postMeeting(user, meeting.ID, args.ChannelId, args.RootId, topic); err != nil {
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

	isAdmin, _ := p.isAuthorizedSysAdmin(user.Id)
	if isAdmin {
		text += "\n" + strings.ReplaceAll(adminHelpText, "|", "`")
	}

	return text, nil
}

// getAutocompleteData retrieves auto-complete data for the "/zoom" command
func (p *Plugin) getAutocompleteData() *model.AutocompleteData {
	if p.getConfiguration().IsValid() != nil {
		zoom := model.NewAutocompleteData("zoom", "[command]", "Available commands: setup")

		setup := model.NewAutocompleteData("setup", "", "Setup the Zoom plugin")
		setup.RoleID = model.SystemAdminRoleId
		zoom.AddCommand(setup)

		return zoom
	}

	available := "start, help"
	if p.configuration.EnableOAuth && !p.configuration.AccountLevelApp {
		available = "start, connect, disconnect, help"
	}
	zoom := model.NewAutocompleteData("zoom", "[command]", fmt.Sprintf("Available commands: %s", available))

	start := model.NewAutocompleteData("start", "[meeting topic]", "Starts a Zoom meeting")
	zoom.AddCommand(start)

	setup := model.NewAutocompleteData("setup", "", "Setup the Zoom plugin")
	setup.RoleID = model.SystemAdminRoleId
	zoom.AddCommand(setup)

	// no point in showing the 'disconnect' option if OAuth is not enabled
	if p.configuration.EnableOAuth && !p.configuration.AccountLevelApp {
		connect := model.NewAutocompleteData("connect", "", "Connect to Zoom")
		disconnect := model.NewAutocompleteData("disconnect", "", "Disonnects from Zoom")
		zoom.AddCommand(connect)
		zoom.AddCommand(disconnect)
	}

	help := model.NewAutocompleteData("help", "", "Display usage")
	zoom.AddCommand(help)

	return zoom
}
