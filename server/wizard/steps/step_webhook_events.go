package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepTitleWebhookEvents = "Select webhook events"

	stepDescriptionWebhookEvents = `Click **Add events** and select the following events:

- Meeting
	- End Meeting
	- Participant/Host joined meeting
	- Participant/Host left meeting
- Recording
	- All recordings have completed

* Click **Save**
* Click **Continue**

<image>`
)

func WebhookEventsStep() steps.Step {
	return steps.NewCustomStepBuilder(stepTitleWebhookEvents, stepDescriptionWebhookEvents).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
