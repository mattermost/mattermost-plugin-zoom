package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameRedirectURL = "redirect_url"

	stepTitleRedirectURL = "Set OAuth redirect URL in Zoom"

	stepDescriptionRedirectURL = `1. In the **Redirect URL for OAuth** input, enter: %s
2. In the **OAuth allow list** at the bottom of the page, enter the same URL: %s

%s`
)

func RedirectURLStep(pluginURL string) steps.Step {
	redirectImage := imagePathToMarkdown(pluginURL, "Redirect URL", "app_credentials.png")

	oauthURL := pluginURL + "/oauth2/complete"
	description := fmt.Sprintf(stepDescriptionRedirectURL, oauthURL, oauthURL, redirectImage)

	return steps.NewCustomStepBuilder(stepNameRedirectURL, stepTitleRedirectURL, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
