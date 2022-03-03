package steps

import (
	"github.com/pkg/errors"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"

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

func VanityURLStep(getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) flow.Step {
	conf := getConfiguration()

	vanityURLDialog := model.Dialog{
		Title:            "",
		IntroductionText: "",
		SubmitLabel:      "Submit",
		Elements: []model.DialogElement{
			{
				DisplayName: "Zoom URL",
				Name:        confURL,
				Default:     conf.ZoomURL,
				Placeholder: "https://yourcompany.zoom.us",
				HelpText:    "The URL for your organization's Zoom instance",
				Type:        "text",
				SubType:     "url",
			},
			{
				DisplayName: "Zoom API URL",
				Name:        confAPI,
				Default:     conf.ZoomAPIURL,
				Placeholder: "https://api.yourcompany.zoom.us/v2",
				HelpText:    "The API URL for your organization's Zoom instance",
				Type:        "text",
				SubType:     "url",
			},
		},
	}

	return flow.NewStep(stepNameVanityURL).
		WithTitle(stepTitleVanityURL).
		WithText(stepDescriptionVanityURL).
		WithButton(flow.Button{
			Name:   "Yes",
			Color:  flow.ColorPrimary,
			Dialog: &vanityURLDialog,
			OnDialogSubmit: func(f *flow.Flow, submission map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
				errors, err := submitVanityURLStep(submission, getConfiguration, client)
				return "", nil, errors, err
			},
		}).
		WithButton(flow.Button{
			Name:    "No",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(""),
		}).
		WithButton(cancelSetupButton)
}

func submitVanityURLStep(submission map[string]interface{}, getConfiguration config.GetConfigurationFunc, client *pluginapi.Client) (map[string]string, error) {
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
		return errorList, nil
	}

	config := getConfiguration()
	config.ZoomURL = baseURL
	config.ZoomAPIURL = apiURL

	err = client.Configuration.SavePluginConfig(config.ToMap())
	if err != nil {
		return nil, errors.Wrap(err, "failed to save plugin config")
	}

	return nil, nil
}
