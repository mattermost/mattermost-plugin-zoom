package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameOAuthScopes = "oauth_scopes"

	stepTitleOAuthScopes = "### :white_check_mark: Step 6: Select OAuth scopes"

	stepDescriptionOAuthScopes = "In the **Scopes** tab, click \"Add Scopes\" button, and select the following OAuth scopes:\n\n" +

		"- `meeting:read` (should already be selected)\n" +
		"- `meeting:write`\n" +
		"- `user:read`\n\n" +

		"Click **Continue**"
)

func OAuthScopesStep(pluginURL string) flow.Step {
	meetingOauthScopeImage := wizardImagePath("oauth_scope_meeting.png")

	return flow.NewStep(stepNameOAuthScopes).
		WithPretext(stepTitleOAuthScopes).
		WithText(stepDescriptionOAuthScopes).
		WithImage(pluginURL, meetingOauthScopeImage).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
