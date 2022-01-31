package steps

import (
	"fmt"
	"net/url"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
)

const (
	stepNameWebhookConfiguration = "webhook_configuration"

	stepTitleWebhookConfiguration = "Configure webhook in Zoom"

	stepDescriptionWebhookConfiguration = `1. Click on the **Feature** category in the left sidebar.
	2. Enable **Event Subscriptions**.
	3. Click **Add New Event Subscription** and give it a name \(e.g. "Mattermost events"\).
	4. Enter in **Event notification endpoint URL**: %s
	5. For the **Event notification receiver** field, select "Only users installed this app" TODO: Is this right?

	<image>

	We'll select the webhook events in the next step.
`
)

func WebhookConfigurationStep(getConfiguration config.GetConfigurationFunc, pluginURL string) steps.Step {
	secret := getConfiguration().WebhookSecret
	secret = url.QueryEscape(secret)

	webhookURL := fmt.Sprintf("%s/webhook?secret=%s", pluginURL, secret)
	description := fmt.Sprintf(stepDescriptionWebhookConfiguration, webhookURL)

	return steps.NewCustomStepBuilder(stepNameWebhookConfiguration, stepTitleWebhookConfiguration, description).
		WithButton(steps.Button{
			Name:  "Continue",
			Style: steps.Default,
		}).
		Build()
}
