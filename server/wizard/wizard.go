package wizard

import (
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"

	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
	steps_local "github.com/mattermost/mattermost-plugin-zoom/server/wizard/steps"
)

type FlowManager struct {
	client           *pluginapi.Client
	getConfiguration config.GetConfigurationFunc
	pluginURL        string
	botUserID        string
	router           *mux.Router

	tracker telemetry.Tracker
}

func NewFlowManager(getConfiguration config.GetConfigurationFunc, client *pluginapi.Client, tracker telemetry.Tracker, router *mux.Router, pluginURL, botUserID string) *FlowManager {
	fm := &FlowManager{
		client:           client,
		pluginURL:        pluginURL,
		botUserID:        botUserID,
		getConfiguration: getConfiguration,
		router:           router,
		tracker:          tracker,
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
		steps_local.FinishedStep(fm.pluginURL).OnRender(func(f *flow.Flow) {
			fm.trackCompleteSetupWizard(f.UserID)
		}),
	}

	return flow.NewFlow(
		"setup",
		fm.client,
		fm.pluginURL,
		fm.botUserID,
	).WithSteps(steps...).ForUser(userID).InitHTTP(fm.router)
}

func (fm *FlowManager) StartConfigurationWizard(userID string) error {
	err := fm.GetConfigurationFlow(userID).Start(flow.State{})
	if err != nil {
		return errors.Wrap(err, "failed to start configuration wizard")
	}

	fm.trackStartSetupWizard(userID)

	return nil
}

func (fm *FlowManager) trackStartSetupWizard(userID string) {
	_ = fm.tracker.TrackUserEvent("setup_wizard_start", userID, map[string]interface{}{
		// TODO: Add more info here
		// "from_invite": fromInvite,
		"time": model.GetMillis(),
	})
}

func (fm *FlowManager) trackCompleteSetupWizard(userID string) {
	_ = fm.tracker.TrackUserEvent("setup_wizard_complete", userID, map[string]interface{}{
		// TODO: Add more info here
		"time": model.GetMillis(),
	})
}
