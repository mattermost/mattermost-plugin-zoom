package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepNameFinished = "finished"

	stepTitleFinished = "Setup finished"

	stepDescriptionFinished = `You're all done!

Nothing needs to be done on the **Activation** page. You can close your browser tab.

Click [here](%s) connect your Zoom account.`
)

func FinishedStep(pluginURL string) steps.Step {
	connectURL := fmt.Sprintf("%s/oauth2/connect", pluginURL)
	description := fmt.Sprintf(stepDescriptionFinished, connectURL)

	return steps.NewEmptyStep(stepNameFinished, stepTitleFinished, description)
}
