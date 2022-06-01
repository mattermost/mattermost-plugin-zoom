package steps

import (
	"fmt"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameFinished = "finished"

	stepTitleFinished = "Setup finished"

	stepDescriptionFinished = `You're all done!

We've sent the announcement to ~{{ .ChannelName }}

Click [here](%s) connect your Zoom account.`
)

func FinishedStep(pluginURL string) flow.Step {
	connectURL := fmt.Sprintf("%s/oauth2/connect", pluginURL)
	description := fmt.Sprintf(stepDescriptionFinished, connectURL)

	return flow.NewStep(stepNameFinished).
		WithTitle(stepTitleFinished).
		WithText(description)
}
