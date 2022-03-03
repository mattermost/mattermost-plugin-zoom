package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameGreeting = "greeting"

	stepPretextGreeting = ":wave: Welcome to your Zoom integration! [Learn more](https://mattermost.gitbook.io/plugin-zoom)"

	stepTitleGreeting = ""

	stepDescriptionGreeting = `Just a few more configuration steps to go!

- Step 1: Register an OAuth application in Zoom.
- Step 2: Configure your OAuth application to work with Mattermost.
- Step 3: Set OAuth redirect URL in Zoom.
- Step 4: Configure a webhook in Zoom.
- Step 5: Add user scopes to the app.

Are you able to set up the integration with a Zoom admin account?`
)

func GreetingStep() flow.Step {
	return flow.NewStep(stepNameGreeting).
		WithPretext(stepPretextGreeting).
		WithTitle(stepTitleGreeting).
		WithText(stepDescriptionGreeting).
		WithButton(continueButton).
		WithButton(flow.Button{
			Name:    "Not now",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(stepNameCanceled),
		})
}
