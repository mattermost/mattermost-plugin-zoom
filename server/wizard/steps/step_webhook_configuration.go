package steps

import (
	"fmt"
	"net/url"

	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
)

const (
	stepNameWebhookConfiguration = "webhook_configuration"

	stepTitleWebhookConfiguration = "Configure webhook in Zoom"

	stepDescriptionWebhookConfiguration = `1. Click on the **Feature** category in the left sidebar.
2. Enable **Event Subscriptions**.
3. Click **Add New Event Subscription** and give it a name \(e.g. "Mattermost events"\).
4. Enter in **Event notification endpoint URL**
5. For the **Event notification receiver** field, select "All users in the account"

We'll select the webhook events in the next step.
`
)

func WebhookConfigurationStep(pluginURL string, getConfiguration config.GetConfigurationFunc) flow.Step {
	secret := getConfiguration().WebhookSecret
	secret = url.QueryEscape(secret)

	eventConfigImage := wizardImagePath("event_configuration.png")

	webhookURL := fmt.Sprintf("%s/webhook?secret=%s", pluginURL, secret)

	webhookURLDialog := model.Dialog{
		Title:            "Webhook URL",
		IntroductionText: "",
		SubmitLabel:      "Continue",
		Elements: []model.DialogElement{
			{
				DisplayName: "",
				Name:        "webhook_url",
				Type:        "text",
				Default:     webhookURL,
				HelpText:    "Copy this URL into Zoom",
				Optional:    true,
			},
		},
	}

	return flow.NewStep(stepNameWebhookConfiguration).
		WithTitle(stepTitleWebhookConfiguration).
		WithText(stepDescriptionWebhookConfiguration).
		WithImage(pluginURL, eventConfigImage).
		WithButton(flow.Button{
			Name:   "Show Webhook URL",
			Color:  flow.ColorPrimary,
			Dialog: &webhookURLDialog,
			OnDialogSubmit: func(f *flow.Flow, submission map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
				return "", nil, nil, nil
			},
		}).
		WithButton(cancelSetupButton)
}
