package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameGotoZoomMarketplace = "goto_zoom_marketplace"

	stepTitleGotoZoomMarketplace = "Let's create an app in Zoom!"

	stepDescriptionGotoZoomMarketplace = `Next we're going to create a new app in your Zoom account.

1. Go to [https://marketplace.zoom.us](https://marketplace.zoom.us) and log in using a Zoom admin account.
2. In the top right corner of the screen, select **Develop** and then **Build App**.

<image>

3. Select **OAuth** in **Choose your app type** section.

<image>`
)

func ZoomMarketplaceStep() steps.Step {
	return steps.NewCustomStepBuilder(stepNameGotoZoomMarketplace, stepTitleGotoZoomMarketplace, stepDescriptionGotoZoomMarketplace).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
