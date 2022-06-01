package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameCreateApp = "create_app"

	stepTitleCreateApp = "### :white_check_mark: Step 1: Register an OAuth application in Zoom"

	stepDescriptionCreateApp = `1. In a browser, go to https://marketplace.zoom.us, then log in with admin credentials.
	2. Select **Develop** in the top right corner, then select **Build App**.

	3. Select **OAuth** in the **Choose your app type** section.
	4. Enter a **name** for your app, such as "Mattermost Plugin".
	5. Choose **User-managed app** as the app type. This means that Mattermost users will need to explicitly connect their account. If you would like to set up an Account-level app, please read this [documentation](https://mattermost.gitbook.io/plugin-zoom/installation/zoom-configuration/zoom-setup-oauth).
	6. When prompted to publish the app on the Zoom Marketplace, select **No**.
	7. Click **Create**.`
)

func CreateZoomAppStep(pluginURL string) flow.Step {
	appTypeImage := wizardImagePath("choose_app_type.png")

	return flow.NewStep(stepNameCreateApp).
		WithPretext(stepTitleCreateApp).
		WithText(stepDescriptionCreateApp).
		WithImage(pluginURL, appTypeImage).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
