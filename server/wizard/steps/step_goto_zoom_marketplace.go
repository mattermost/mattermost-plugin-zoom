package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameGotoZoomMarketplace = "goto_zoom_marketplace"

	stepTitleGotoZoomMarketplace = "Let's create an app in Zoom!"

	stepDescriptionGotoZoomMarketplace = `Next we're going to create a new app in your Zoom account.

1. Go to [https://marketplace.zoom.us](https://marketplace.zoom.us) and log in using a Zoom admin account.
2. In the top right corner of the screen, select **Develop** and then **Build App**.

%s

3. Select **OAuth** in **Choose your app type** section.

%s`
)

func imagePathToMarkdown(pluginURL, name, imgPath string) string {
	return fmt.Sprintf("![%s](%s/public/setup_flow_images/%s)", name, pluginURL, imgPath)
}

func ZoomMarketplaceStep(pluginURL string) steps.Step {
	// TODO: site URL

	buildAppImage := imagePathToMarkdown(pluginURL, "Build Zoom App", "build_app.png")
	appTypeImage := imagePathToMarkdown(pluginURL, "Choose App Type", "choose_app_type.png")

	description := fmt.Sprintf(stepDescriptionGotoZoomMarketplace, buildAppImage, appTypeImage)

	return steps.NewCustomStepBuilder(stepNameGotoZoomMarketplace, stepTitleGotoZoomMarketplace, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
