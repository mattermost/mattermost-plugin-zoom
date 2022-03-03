package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameOAuthScopes = "oauth_scopes"

	stepTitleOAuthScopes = "Select OAuth scopes"

	stepDescriptionOAuthScopes = `In the **Scopes** tab, click "Add Scopes" button, and select the following OAuth scopes:

- meeting:read (should already be selected)
- meeting:write
- user:read

Click **Continue**`
)

func OAuthScopesStep(pluginURL string) flow.Step {
	meetingOauthScopeImage := wizardImagePath("oauth_scope_meeting.png")

	return flow.NewStep(stepNameOAuthScopes).
		WithTitle(stepTitleOAuthScopes).
		WithText(stepDescriptionOAuthScopes).
		WithImage(pluginURL, meetingOauthScopeImage).
		WithButton(continueButton).
		WithButton(cancelSetupButton)
}
