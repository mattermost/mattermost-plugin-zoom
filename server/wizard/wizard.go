package wizard

import (
	"github.com/gorilla/mux"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
	steps_local "github.com/mattermost/mattermost-plugin-zoom/server/wizard/steps"
)

type FlowManager struct {
	client           *pluginapi.Client
	getConfiguration config.GetConfigurationFunc
	pluginURL        string
	botUserID        string
	router           *mux.Router
}

func NewFlowManager(getConfiguration config.GetConfigurationFunc, client *pluginapi.Client, router *mux.Router, pluginURL, botUserID string) *FlowManager {
	fm := &FlowManager{
		client:           client,
		pluginURL:        pluginURL,
		botUserID:        botUserID,
		getConfiguration: getConfiguration,
		router:           router,
	}

	return fm
}

func (fm *FlowManager) GetConfigurationFlow(userID string) *flow.Flow {
	steps := []flow.Step{
		steps_local.GreetingStep(),
		steps_local.VanityURLStep(fm.getConfiguration, fm.client),
		steps_local.ZoomMarketplaceStep(fm.pluginURL),
		steps_local.CreateZoomAppStep(fm.pluginURL),
		steps_local.ZoomAppCredentialsStep(fm.pluginURL, fm.getConfiguration, fm.client),
		steps_local.RedirectURLStep(fm.pluginURL),
		steps_local.WebhookConfigurationStep(fm.pluginURL, fm.getConfiguration),
		steps_local.WebhookEventsStep(fm.pluginURL),
		steps_local.OAuthScopesStep(fm.pluginURL),
		steps_local.FinishedStep(fm.pluginURL),
	}

	return flow.NewFlow(
		"setup",
		fm.client,
		fm.pluginURL,
		fm.botUserID,
	).WithSteps(steps...).ForUser(userID).InitHTTP(fm.router)
}

func (fm *FlowManager) StartConfigurationWizard(userID string) error {
	return fm.GetConfigurationFlow(userID).Start(flow.State{})
}
