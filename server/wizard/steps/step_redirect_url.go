package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameRedirectURL = "redirect_url"

	stepTitleRedirectURL = "Set OAuth redirect URL in Zoom"

	stepDescriptionRedirectURL = `1. In the **Redirect URL for OAuth** input, enter: %s
2. In the **OAuth allow list** at the bottom of the page, enter the same URL: %s`
)

func RedirectURLStep(pluginURL string) flow.Step {
	redirectImage := wizardImagePath("app_credentials.png")

	oauthURL := fmt.Sprintf("`%s/oauth2/complete`", pluginURL)
	description := fmt.Sprintf(stepDescriptionRedirectURL, oauthURL, oauthURL)

	return flow.NewStep(stepNameRedirectURL).
		WithTitle(stepTitleRedirectURL).
		WithText(description).
		WithImage(pluginURL, redirectImage).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
