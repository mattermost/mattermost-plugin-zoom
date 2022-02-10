package steps

import (
	"github.com/pkg/errors"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
)

const (
	stepNameVanityURL = "vanity_url"

	stepTitleVanityURL = "Zoom Vanity URL"

	stepDescriptionVanityURL = "Are you using your own [Vanity URL](https://support.zoom.us/hc/en-us/articles/215062646-Guidelines-for-Vanity-URL-requests) (sub-domain) for your Zoom account?"

	confURL = "ZoomURL"
	confAPI = "ZoomAPIURL"
)

func VanityURLStep(getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) steps.Step {
	return steps.NewCustomStepBuilder(stepNameVanityURL, stepTitleVanityURL, stepDescriptionVanityURL).
		WithButton(steps.Button{
			Name:  "Yes",
			Style: steps.Primary,
			Dialog: &steps.Dialog{
				Dialog: selfHostedDialog,
				OnDialogSubmit: func(userID string, submission map[string]interface{}) (int, *steps.Attachment, string, map[string]string) {
					res, errors := submitSelfHostedStep(submission, getConfiguration, client)
					return 0, nil, res, errors
				},
			},
			OnClick: func(notSure string) int {
				return -1
			},
		}).
		WithButton(steps.Button{
			Name:  "No",
			Style: steps.Default,
		}).
		Build()
}

var selfHostedDialog = model.Dialog{
	Title:            "",
	IntroductionText: "",
	SubmitLabel:      "Submit",
	Elements: []model.DialogElement{
		{
			DisplayName: "Zoom URL",
			Name:        confURL,
			Placeholder: "https://yourcompany.zoom.us",
			HelpText:    "The URL for your organization's Zoom instance",
			Type:        "text",
			SubType:     "url",
		},
		{
			DisplayName: "Zoom API URL",
			Name:        confAPI,
			Placeholder: "https://api.yourcompany.zoom.us/v2",
			HelpText:    "The API URL for your organization's Zoom instance",
			Type:        "text",
			SubType:     "url",
		},
	},
}

func submitSelfHostedStep(submission map[string]interface{}, getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) (string, map[string]string) {
	errorList := map[string]string{}

	baseURL, err := isValidURLSubmission(submission, confURL)
	if err != nil {
		errorList[confURL] = err.Error()
	}

	apiURL, err := isValidURLSubmission(submission, confAPI)
	if err != nil {
		errorList[confAPI] = err.Error()
	}

	if len(errorList) != 0 {
		return "", errorList
	}

	config := getConfiguration()
	config.ZoomURL = baseURL
	config.ZoomAPIURL = apiURL

	err = client.Configuration.SavePluginConfig(config.ToMap())
	if err != nil {
		return errors.Wrap(err, "failed to save plugin config").Error(), nil
	}

	return "", nil
}
