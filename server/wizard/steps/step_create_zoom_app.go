package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameCreateApp = "create_app"

	stepTitleCreateApp = "Create Zoom app"

	stepDescriptionCreateApp = `1. Enter a name for your app, such as "Mattermost Plugin".
2. Choose **User-managed app** as the app type.
3. Choose **No** for **Would like to publish this app on Zoom Marketplace**.
4. Click **Create**.

%s`
)

func CreateZoomAppStep(pluginURL string) steps.Step {
	createAppImage := imagePathToMarkdown(pluginURL, "Create OAuth App", "create_oauth_app.png")

	description := fmt.Sprintf(stepDescriptionCreateApp, createAppImage)

	return steps.NewCustomStepBuilder(stepNameCreateApp, stepTitleCreateApp, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
