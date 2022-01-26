package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepTitleOAuthScopes = "Select OAuth scopes"

	stepDescriptionOAuthScopes = `In the **Scopes** tab, select these OAuth scopes:

- meeting:write
- user:read

Click **Continue**

<image>`
)

func OAuthScopesStep() steps.Step {
	return steps.NewCustomStepBuilder(stepTitleOAuthScopes, stepDescriptionOAuthScopes).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
