package wizard

import (
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-api/experimental/flow"
	"github.com/mattermost/mattermost-plugin-api/experimental/telemetry"

	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-plugin-zoom/server/config"
	"github.com/mattermost/mattermost-plugin-zoom/server/wizard/steps"
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

var configurationFlow *flow.Flow

func (fm *FlowManager) GetConfigurationFlow() *flow.Flow {
	if configurationFlow != nil {
		return configurationFlow
	}

	steps := []flow.Step{
		steps.GreetingStep().
			OnRender(fm.trackEvent("setup_wizard_step_greeting")),

		steps.VanityURLStep(fm.getConfiguration, fm.client).
			OnRender(fm.trackEvent("setup_wizard_step_vanity_url")),

		steps.CreateZoomAppStep(fm.pluginURL).
			OnRender(fm.trackEvent("setup_wizard_step_zoom_app_creation")),

		steps.ZoomAppCredentialsStep(fm.pluginURL, fm.getConfiguration, fm.client).
			OnRender(fm.trackEvent("setup_wizard_step_zoom_app_credentials")),

		steps.RedirectURLStep(fm.pluginURL).
			OnRender(fm.trackEvent("setup_wizard_step_redirect_url")),

		steps.WebhookConfigurationStep(fm.pluginURL, fm.getConfiguration).
			OnRender(fm.trackEvent("setup_wizard_step_webhook_configuration")),

		steps.WebhookEventsStep(fm.pluginURL).
			OnRender(fm.trackEvent("setup_wizard_step_webhook_events")),

		steps.OAuthScopesStep(fm.pluginURL).
			OnRender(fm.trackEvent("setup_wizard_step_oauth_scopes")),

		steps.AnnouncementQuestionStep(fm.client).
			OnRender(fm.trackEvent("setup_wizard_step_announcement_question")),

		steps.FinishedStep(fm.pluginURL).Terminal().
			OnRender(fm.trackEvent("setup_wizard_finished")),

		steps.CanceledStep().Terminal().
			OnRender(fm.trackEvent("setup_wizard_step_canceled")),
	}

	return flow.NewFlow(
		"setup",
		fm.client,
		fm.pluginURL,
		fm.botUserID,
	).
		WithSteps(steps...).
		InitHTTP(fm.router)
}

func (fm *FlowManager) StartConfigurationWizard(userID string) error {
	err := fm.GetConfigurationFlow().ForUser(userID).Start(flow.State{})
	if err != nil {
		return errors.Wrap(err, "failed to start configuration wizard")
	}

	fm.trackStartSetupWizard(userID)

	return nil
}

func (fm *FlowManager) trackEvent(eventName string) func(f *flow.Flow) {
	return func(f *flow.Flow) {
		state := map[string]interface{}{}
		state["time"] = model.GetMillis()

		_ = fm.tracker.TrackUserEvent(eventName, f.UserID, state)
	}
}

func (fm *FlowManager) trackStartSetupWizard(userID string) {
	_ = fm.tracker.TrackUserEvent("setup_wizard_start", userID, map[string]interface{}{
		"time": model.GetMillis(),
	})
}
