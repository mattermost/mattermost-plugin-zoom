package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameFinished = "finished"

	stepTitleFinished = "Setup finished"

	stepDescriptionFinished = `You're all done!

Nothing needs to be done on the **Activation** page. You can close your browser tab.

Click [here](%s) connect your Zoom account.`
)

func FinishedStep(pluginURL string) flow.Step {
	connectURL := fmt.Sprintf("%s/oauth2/connect", pluginURL)
	description := fmt.Sprintf(stepDescriptionFinished, connectURL)

	return flow.NewStep(stepNameFinished).
		WithTitle(stepTitleFinished).
		WithText(description)
}
