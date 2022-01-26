package wizard

import (
	"github.com/gorilla/mux"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/bot/logger"
	"github.com/mattermost/mattermost-plugin-api/experimental/bot/poster"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow/steps"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
	steps_local "github.com/mattermost/mattermost-plugin-zoom/server/wizard/steps"
)

type FlowManager struct {
	client           *pluginapi.Client
	getConfiguration config.GetConfigurationFunc
	pluginURL        string

	wizardController flow.Controller
}

type propertyStore struct {
	props map[string]interface{}
}

func (ps *propertyStore) SetProperty(userID string, propertyName string, value interface{}) error {
	ps.props[userID+propertyName] = value
	return nil
}

func NewFlowManager(getConfiguration config.GetConfigurationFunc, client *pluginapi.Client, router *mux.Router, logger logger.Logger, pluginURL, botUserID string) *FlowManager {
	fm := &FlowManager{
		client:           client,
		pluginURL:        pluginURL,
		getConfiguration: getConfiguration,
	}

	fm.wizardController = flow.NewFlowController(
		logger,
		router,
		poster.NewPoster(&client.Post, botUserID),
		&client.Frontend,
		fm.pluginURL,
		fm.GetConfigurationFlow(),
		flow.NewFlowStore(*client, "flow_store"),
		&propertyStore{},
	)

	return fm
}

func (fm *FlowManager) GetConfigurationFlow() flow.Flow {
	steps := []steps.Step{
		steps_local.GreetingStep(),
		steps_local.SelfHostedQuestionStep(fm.getConfiguration, fm.client),
		steps_local.ZoomMarketplaceStep(),
		steps_local.CreateZoomAppStep(),
		steps_local.ZoomAppCredentialsStep(fm.getConfiguration, fm.client),
		steps_local.RedirectURLStep(fm.pluginURL),
		steps_local.WebhookConfigurationStep(fm.getConfiguration, fm.pluginURL),
		steps_local.WebhookEventsStep(),
		steps_local.OAuthScopesStep(),
		steps_local.FinishedStep(fm.pluginURL),
	}

	f := flow.NewFlow(steps, "/wizard", nil)
	return f
}

func (fm *FlowManager) StartConfigurationWizard(userID string) error {
	return fm.wizardController.Start(userID)
}
