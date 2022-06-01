package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameWebhookEvents = "webhook_events"

	stepTitleWebhookEvents = "### :white_check_mark: Step 5: Add webhook events"

	stepDescriptionWebhookEvents = `- Click **Add events** and select the "End Meeting" event
- Then click **Done**, **Save**, and **Continue**`
)

func WebhookEventsStep(pluginURL string) flow.Step {
	meetingEventTypesImage := wizardImagePath("event_type_meeting.png")

	return flow.NewStep(stepNameWebhookEvents).
		WithPretext(stepTitleWebhookEvents).
		WithText(stepDescriptionWebhookEvents).
		WithImage(pluginURL, meetingEventTypesImage).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
