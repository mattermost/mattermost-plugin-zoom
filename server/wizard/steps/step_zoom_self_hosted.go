package steps

import (
	"github.com/pkg/errors"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
)

const (
	stepNameSelfHostedQuestion = "self_hosted_question"

	stepTitleSelfHostedQuestion = ""

	stepDescriptionSelfHostedQuestion = "Are you using a self-hosted private cloud or on-prem Zoom server?"

	confURL = "ZoomURL"
	confAPI = "ZoomAPIURL"
)

func SelfHostedQuestionStep(getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) steps.Step {
	return steps.NewCustomStepBuilder(stepNameSelfHostedQuestion, stepTitleSelfHostedQuestion, stepDescriptionSelfHostedQuestion).
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
			Placeholder: "https://yourzoom.com",
			HelpText:    "The URL for a self-hosted private cloud or on-prem Zoom server",
			Type:        "text",
			SubType:     "url",
		},
		{
			DisplayName: "Zoom API URL",
			Name:        confAPI,
			Placeholder: "https://api.yourzoom.com/v2",
			HelpText:    "The API URL for a self-hosted private cloud or on-prem Zoom server",
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
