package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameOAuthScopes = "oauth_scopes"

	stepTitleOAuthScopes = "Select OAuth scopes"

	stepDescriptionOAuthScopes = `In the **Scopes** tab, click "Add Scopes" button, and select the following OAuth scopes:

- meeting:read (should already be selected)
- meeting:write
- user:read

Click **Continue**

%s`
)

func OAuthScopesStep(pluginURL string) flow.Step {
	meetingOauthScopeImage := imagePathToMarkdown(pluginURL, "Meeting OAuth Scope", "oauth_scope_meeting.png")

	description := fmt.Sprintf(stepDescriptionOAuthScopes, meetingOauthScopeImage)

	return flow.NewStep(stepNameOAuthScopes).
		WithTitle(stepTitleOAuthScopes).
		WithText(description).
		WithButton(flow.Button{
			Name:    "Continue",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(""),
		})
}
