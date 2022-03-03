package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameCreateApp = "create_app"

	stepTitleCreateApp = "Create Zoom app"

	stepDescriptionCreateApp = `1. Enter a name for your app, such as "Mattermost Plugin".
2. Choose **User-managed app** as the app type.
3. Choose **No** for **Would like to publish this app on Zoom Marketplace**.
4. Click **Create**.`
)

func CreateZoomAppStep(pluginURL string) flow.Step {
	return flow.NewStep(stepNameCreateApp).
		WithTitle(stepTitleCreateApp).
		WithImage(pluginURL, "public/setup_flow_images/create_oauth_app.png").
		WithText(stepDescriptionCreateApp).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
