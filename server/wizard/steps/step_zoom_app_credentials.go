package steps

import (
	"github.com/pkg/errors"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"

	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
)

const (
	stepNameZoomAppCredentials = "zoom_app_credentials"

	stepTitleZoomAppCredentials = "Enter Zoom app credentials"

	stepDescriptionZoomAppCredentials = `In the **App Credentials** tab, note the values for Client ID and Client secret.

Click the button below to open a dialog to enter these two values.

%s`

	confClientID     = "client_id"
	confClientSecret = "client_secret"
)

func ZoomAppCredentialsStep(pluginURL string, getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) flow.Step {
	appCredentialsImage := wizardImagePath("app_credentials.png")

	return flow.NewStep(stepNameZoomAppCredentials).
		WithTitle(stepTitleZoomAppCredentials).
		WithText(stepDescriptionZoomAppCredentials).
		WithImage(pluginURL, appCredentialsImage).
		WithButton(flow.Button{
			Name:   "Enter Client ID and Client secret",
			Color:  flow.ColorPrimary,
			Dialog: &zoomAppCredentialsDialog,
			OnDialogSubmit: func(f *flow.Flow, submission map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
				errors, err := submitZoomAppCredentialsStep(submission, getConfiguration, client)
				return "", nil, errors, err
			},
		}).
		WithButton(cancelSetupButton)
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
			SubType:     "text",
		},
	},
}

func submitZoomAppCredentialsStep(submission map[string]interface{}, getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) (map[string]string, error) {
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
		return errorList, nil
	}

	config := getConfiguration()
	config.OAuthClientID = clientID
	config.OAuthClientSecret = clientSecret

	err = client.Configuration.SavePluginConfig(config.ToMap())
	if err != nil {
		return nil, errors.Wrap(err, "failed to save plugin config")
	}

	return nil, nil
}
