package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameCreateApp = "create_app"

	stepTitleCreateApp = ""

	stepDescriptionCreateApp = `1. Enter a name for your app, such as "Mattermost Plugin".
2. Choose **Account-level app** as the app type.
3. Choose **No** for **Would like to publish this app on Zoom Marketplace**.
4. Click **Create**.

<image>`
)

func CreateZoomAppStep() steps.Step {
	return steps.NewCustomStepBuilder(stepNameCreateApp, stepTitleCreateApp, stepDescriptionCreateApp).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
