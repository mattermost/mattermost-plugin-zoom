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

<image>`
)

func RedirectURLStep(pluginURL string) steps.Step {
	oauthURL := pluginURL + "/oauth2/complete"
	description := fmt.Sprintf(stepDescriptionRedirectURL, oauthURL, oauthURL)

	return steps.NewCustomStepBuilder(stepNameRedirectURL, stepTitleRedirectURL, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
