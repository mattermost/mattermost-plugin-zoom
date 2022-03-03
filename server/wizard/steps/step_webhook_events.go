package steps

import (
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
	meetingEventTypesImage := wizardImagePath("event_type_meeting.png")

	return flow.NewStep(stepNameWebhookEvents).
		WithTitle(stepTitleWebhookEvents).
		WithText(stepDescriptionWebhookEvents).
		WithImage(pluginURL, meetingEventTypesImage).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
