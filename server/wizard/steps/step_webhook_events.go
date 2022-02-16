package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameWebhookEvents = "webhook_events"

	stepTitleWebhookEvents = "Select webhook events"

	stepDescriptionWebhookEvents = `- Click **Add events** and select the "End Meeting" event
- Then click **Done**, **Save**, and **Continue**

%s`
)

func WebhookEventsStep(pluginURL string) flow.Step {
	meetingEventTypesImage := imagePathToMarkdown(pluginURL, "Recording Events", "event_type_meeting.png")

	description := fmt.Sprintf(stepDescriptionWebhookEvents, meetingEventTypesImage)

	return flow.NewStep(stepNameWebhookEvents).
		WithTitle(stepTitleWebhookEvents).
		WithText(description).
		WithButton(flow.Button{
			Name:    "Continue",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(""),
		})
}
