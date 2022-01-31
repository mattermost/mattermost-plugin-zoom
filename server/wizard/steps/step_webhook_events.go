package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameWebhookEvents = "webhook_events"

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

%s

%s`
)

func WebhookEventsStep(pluginURL string) steps.Step {
	recordingEventTypesImage := imagePathToMarkdown(pluginURL, "Meeting Events", "event_type_recording.png")
	meetingEventTypesImage := imagePathToMarkdown(pluginURL, "Recording Events", "event_type_meeting.png")

	description := fmt.Sprintf(stepDescriptionWebhookEvents, meetingEventTypesImage, recordingEventTypesImage)

	return steps.NewCustomStepBuilder(stepNameWebhookEvents, stepTitleWebhookEvents, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
