package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameDelegateQuestion = "delegate question"

	stepPretextDelegateQuestion = ""

	stepTitleDelegateQuestion = ""

	stepDescriptionDelegateQuestion = `:wave: Welcome to your Zoom integration! I'll walk you through the process here, so you can begin using the integration. Feel free to read the [documentation](https://mattermost.gitbook.io/plugin-zoom) if you'd like.

- Step 1: Register an OAuth application in Zoom.
- Step 2: Configure your OAuth application to work with Mattermost.
- Step 3: Set OAuth redirect URL in Zoom.
- Step 4: Configure a webhook in Zoom.
- Step 5: Add user scopes to the app.
Are you able to set up the integration with a Zoom admin account?`
)

func DelegateQuestionStep() flow.Step {
	return flow.NewStep(stepNameDelegateQuestion).
		WithTitle(stepTitleDelegateQuestion).
		WithText(stepDescriptionDelegateQuestion).
		WithButton(continueButton).
		WithButton(flow.Button{
			Name:    "Not now",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(stepNameCanceled),
		})
}
