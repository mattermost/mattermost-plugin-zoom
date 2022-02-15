package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameGreeting = "greeting"

	stepTitleGreeting = ""

	stepDescriptionGreeting = `:wave: Welcome to Zoom for Mattermost! I'll walk you through the process here, so you can begin using the integration. Feel free to read the [documentation](https://mattermost.gitbook.io/plugin-zoom) if you'd like.

Are you able to set up the integration with a Zoom admin account?`
)

func GreetingStep() flow.Step {
	return flow.NewStep(stepNameGreeting).
		WithPretext(stepTitleGreeting).
		WithText(stepDescriptionGreeting).
		WithButton(flow.Button{
			Name:    "Continue",
			Color:   flow.ColorPrimary,
			OnClick: flow.Goto(""),
		}).
		WithButton(flow.Button{
			Name:  "Not now",
			Color: flow.ColorDefault,
			// TODO: go to "you can setup later" step
		})
}
