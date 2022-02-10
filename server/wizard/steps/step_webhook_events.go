package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameWebhookEvents = "webhook_events"

	stepTitleWebhookEvents = "Select webhook events"

	stepDescriptionWebhookEvents = `- Click **Add events** and select the "End Meeting" event
- Then click **Done**, **Save**, and **Continue**

%s`
)

func WebhookEventsStep(pluginURL string) steps.Step {
	meetingEventTypesImage := imagePathToMarkdown(pluginURL, "Recording Events", "event_type_meeting.png")

	description := fmt.Sprintf(stepDescriptionWebhookEvents, meetingEventTypesImage)

	return steps.NewCustomStepBuilder(stepNameWebhookEvents, stepTitleWebhookEvents, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
