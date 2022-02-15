package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameGotoZoomMarketplace = "goto_zoom_marketplace"

	stepTitleGotoZoomMarketplace = "Go to Zoom marketplace"

	stepDescriptionGotoZoomMarketplace = `Next we're going to create a new app in your Zoom account.

1. Go to https://marketplace.zoom.us and log in using a Zoom admin account.
2. In the top right corner of the screen, select **Develop** and then **Build App**.

%s

3. Select **OAuth** in **Choose your app type** section.

%s`
)

func ZoomMarketplaceStep(pluginURL string) flow.Step {
	buildAppImage := imagePathToMarkdown(pluginURL, "Build Zoom App", "build_app.png")
	appTypeImage := imagePathToMarkdown(pluginURL, "Choose App Type", "choose_app_type.png")

	description := fmt.Sprintf(stepDescriptionGotoZoomMarketplace, buildAppImage, appTypeImage)

	return flow.NewStep(stepNameGotoZoomMarketplace).
		WithPretext(stepTitleGotoZoomMarketplace).
		WithText(description).
		WithButton(flow.Button{
			Name:    "Continue",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(""),
		})
}
