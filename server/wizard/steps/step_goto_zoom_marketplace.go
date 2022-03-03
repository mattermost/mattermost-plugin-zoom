package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameGotoZoomMarketplace = "goto_zoom_marketplace"

	stepTitleGotoZoomMarketplace = "Go to Zoom marketplace"

	stepDescriptionGotoZoomMarketplace = `Next we're going to create a new app in your Zoom account.

1. Go to https://marketplace.zoom.us and log in using a Zoom admin account.
2. In the top right corner of the screen, select **Develop** and then **Build App**.

3. Select **OAuth** in **Choose your app type** section.`
)

func ZoomMarketplaceStep(pluginURL string) flow.Step {
	appTypeImage := wizardImagePath("choose_app_type.png")

	return flow.NewStep(stepNameGotoZoomMarketplace).
		WithTitle(stepTitleGotoZoomMarketplace).
		WithText(stepDescriptionGotoZoomMarketplace).
		WithImage(pluginURL, appTypeImage).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
