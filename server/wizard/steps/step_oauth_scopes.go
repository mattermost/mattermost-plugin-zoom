package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameOAuthScopes = "oauth_scopes"

	stepTitleOAuthScopes = "Select OAuth scopes"

	stepDescriptionOAuthScopes = `In the **Scopes** tab, select these OAuth scopes:

- meeting:write
- user:read

Click **Continue**

%s

%s`
)

func OAuthScopesStep() steps.Step {
	meetingOauthScopeImage := imagePathToMarkdown("Meeting OAuth Scope", "oauth_scope_meeting.png")
	recordingOauthScopeImage := imagePathToMarkdown("Recording OAuth Scope", "oauth_scope_recording.png")

	description := fmt.Sprintf(stepDescriptionOAuthScopes, meetingOauthScopeImage, recordingOauthScopeImage)

	return steps.NewCustomStepBuilder(stepNameOAuthScopes, stepTitleOAuthScopes, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
