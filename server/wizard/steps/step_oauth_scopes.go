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

func OAuthScopesStep(pluginURL string) steps.Step {
	meetingOauthScopeImage := imagePathToMarkdown(pluginURL, "Meeting OAuth Scope", "oauth_scope_meeting.png")
	recordingOauthScopeImage := imagePathToMarkdown(pluginURL, "Recording OAuth Scope", "oauth_scope_recording.png")

	description := fmt.Sprintf(stepDescriptionOAuthScopes, meetingOauthScopeImage, recordingOauthScopeImage)

	return steps.NewCustomStepBuilder(stepNameOAuthScopes, stepTitleOAuthScopes, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
