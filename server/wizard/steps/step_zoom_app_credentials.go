package steps

import (
	"emperror.dev/errors"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"
	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
)

const (
	stepTitleZoomAppCredentials = ""

	stepDescriptionZoomAppCredentials = `In the **App Credentials** tab, note the values for Client ID and Client secret.

Click the button below to open a dialog to enter these two values.

<image>`

	confClientID     = "client_id"
	confClientSecret = "client_secret"
)

func ZoomAppCredentialsStep(getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) steps.Step {
	return steps.NewCustomStepBuilder(stepTitleZoomAppCredentials, stepDescriptionZoomAppCredentials).
		WithButton(steps.Button{
			Name:  "Enter Client ID and Client secret",
			Style: steps.Primary,
			Dialog: &steps.Dialog{
				Dialog: zoomAppCredentialsDialog,
				OnDialogSubmit: func(userID string, submission map[string]interface{}) (int, *steps.Attachment, string, map[string]string) {
					res, errors := submitZoomAppCredentialsStep(submission, getConfiguration, client)
					return 0, nil, res, errors
				},
			},
			OnClick: func(notSure string) int {
				return -1
			},
		}).
		Build()
}

var zoomAppCredentialsDialog = model.Dialog{
	Title:            "Enter Zoom credentials",
	IntroductionText: "",
	SubmitLabel:      "Submit",
	Elements: []model.DialogElement{
		{
			DisplayName: "Client ID",
			Name:        confClientID,
			HelpText:    "",
			Type:        "text",
			SubType:     "text",
		},
		{
			DisplayName: "Client Secret",
			Name:        confClientSecret,
			HelpText:    "",
			Type:        "text",
			SubType:     "password",
		},
	},
}

func submitZoomAppCredentialsStep(submission map[string]interface{}, getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) (string, map[string]string) {
	errorList := map[string]string{}

	clientID, err := safeString(submission, confClientID)
	if err != nil {
		errorList[confClientID] = err.Error()
	}

	clientSecret, err := safeString(submission, confClientSecret)
	if err != nil {
		errorList[confClientSecret] = err.Error()
	}

	if len(errorList) != 0 {
		return "", errorList
	}

	config := getConfiguration()
	config.OAuthClientID = clientID
	config.OAuthClientSecret = clientSecret

	err = client.Configuration.SavePluginConfig(config.ToMap())
	if err != nil {
		return errors.Wrap(err, "failed to save plugin config").Error(), nil
	}

	return "", nil
}
