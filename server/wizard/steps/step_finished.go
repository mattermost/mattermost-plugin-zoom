package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
)

const (
	stepTitleFinished = "You're finished setting up the plugin! :tada:"

	stepDescriptionFinished = `You're all done!

Nothing needs to be done on the **Install** page. You can close your browser tab.

Click [here](%s) connect your Zoom account.`
)

func FinishedStep(pluginURL string) steps.Step {
	connectURL := fmt.Sprintf("%s/oauth2/connect", pluginURL)
	description := fmt.Sprintf(stepDescriptionFinished, connectURL)

	return steps.NewEmptyStep(stepTitleFinished, description)
}
