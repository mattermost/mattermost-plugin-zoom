package steps

import (
	"fmt"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
)

const (
	dialogElementAnnouncementChannelID = "channel_id"
	dialogElementAnnouncementMessage   = "message"

	pluginAbilitiesString = "You can now start Zoom meetings by:\n" +
		"- Clicking the the Zoom call button in the channel header, or\n" +
		"- Running the `/zoom start` command\n\n" +
		"See the [documentation](https://mattermost.gitbook.io/plugin-zoom/usage/start-meetings) for details on using the Zoom plugin."

	stepNameAnnouncementQuestion = "AnnouncementQuestion"

	stepTitleAnnouncementQuestion = "##### :tada: Success! You've successfully set up your Mattermost Zoom integration!"

	stepDescriptionAnnouncementQuestion = "%s\n\n" +

		"Want to let your team know?"

	announcementMessage = "We've added an integration that connects Zoom and Mattermost. %s\n\n" +

		"It's easy to get started, run the `/zoom connect` slash command from any channel within Mattermost to connect your user account."
)

func AnnouncementQuestionStep(client *pluginapi.Client) flow.Step {
	description := fmt.Sprintf(stepDescriptionAnnouncementQuestion, pluginAbilitiesString)

	return flow.NewStep(stepNameAnnouncementQuestion).
		WithTitle(stepTitleAnnouncementQuestion).
		WithText(description).
		WithButton(flow.Button{
			Name:   "Send message",
			Color:  flow.ColorPrimary,
			Dialog: &announcementDialog,
			OnDialogSubmit: func(f *flow.Flow, submission map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
				state, errors, err := submitAnnouncementStep(f.UserID, submission, client)
				return "", state, errors, err
			},
		}).
		WithButton(flow.Button{
			Name:    "Not now",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(""),
		})
}

var announcementDialog = model.Dialog{
	Title:       "Notify your team",
	SubmitLabel: "Send message",
	Elements: []model.DialogElement{
		{
			DisplayName: "To",
			Name:        dialogElementAnnouncementChannelID,
			Type:        "select",
			Placeholder: "Select channel",
			DataSource:  "channels",
		},
		{
			DisplayName: "Message",
			Name:        dialogElementAnnouncementMessage,
			Type:        "textarea",
			Default:     fmt.Sprintf(announcementMessage, pluginAbilitiesString),
			HelpText:    "You can edit this message before sending it.",
		},
	},
}

func submitAnnouncementStep(userID string, submission map[string]interface{}, client *pluginapi.Client) (flow.State, map[string]string, error) {
	errorList := map[string]string{}

	channelID, err := safeString(submission, dialogElementAnnouncementChannelID)
	if err != nil {
		errorList[dialogElementAnnouncementChannelID] = err.Error()
	}

	message, err := safeString(submission, dialogElementAnnouncementMessage)
	if err != nil {
		errorList[dialogElementAnnouncementMessage] = err.Error()
	}

	if len(errorList) != 0 {
		return nil, errorList, nil
	}

	channel, err := client.Channel.Get(channelID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get channel")
	}

	post := &model.Post{
		UserId:    userID,
		ChannelId: channelID,
		Message:   message,
	}
	err = client.Post.CreatePost(post)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create announcement post")
	}

	return flow.State{
		"ChannelName": channel.Name,
	}, nil, nil
}
