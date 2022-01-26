package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameGreeting = "greeting"

	stepTitleGreeting = ""

	stepDescriptionGreeting = `:wave: Welcome to Zoom for Mattermost! I'll walk you through the process here, so you can begin using the integration. Feel free to read the [documentation](https://mattermost.gitbook.io/plugin-zoom) if you'd like.

Are you able to set up the integration with a Zoom admin account?`
)

func GreetingStep() steps.Step {
	return steps.NewCustomStepBuilder(stepNameGreeting, stepTitleGreeting, stepDescriptionGreeting).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Primary,
			OnClick: func(userID string) int {
				return 0
			},
		}).
		WithButton(steps.Button{
			Name:  "Not now",
			Style: steps.Default,
			OnClick: func(userID string) int {
				return 999
			},
		}).
		Build()
}
