package steps

import (
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
)

const (
	stepNameCanceled = "Canceled"

	stepTitleCanceled = "Setup canceled"

	stepDescriptionCanceled = "Zoom integration setup has stopped. Restart setup later by running `/zoom setup`"
)

func CanceledStep() flow.Step {
	return flow.NewStep(stepNameCanceled).
		WithTitle(stepTitleCanceled).
		WithText(stepDescriptionCanceled)
}
