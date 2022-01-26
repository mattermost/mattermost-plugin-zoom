package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameOAuthScopes = "oauth_scopes"

	stepTitleOAuthScopes = "Select OAuth scopes"

	stepDescriptionOAuthScopes = `In the **Scopes** tab, select these OAuth scopes:

- meeting:write
- user:read

Click **Continue**

<image>`
)

func OAuthScopesStep() steps.Step {
	return steps.NewCustomStepBuilder(stepNameOAuthScopes, stepTitleOAuthScopes, stepDescriptionOAuthScopes).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
