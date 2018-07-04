package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/schema"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"

	zd "github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

const (
	POST_MEETING_KEY = "post_meeting_"
)

type Plugin struct {
	plugin.MattermostPlugin

	zoomClient *zd.Client

	ZoomURL       string
	ZoomAPIURL    string
	APIKey        string
	APISecret     string
	WebhookSecret string
}

func (p *Plugin) OnActivate() error {
	if err := p.IsConfigurationValid(); err != nil {
		return err
	}

	p.zoomClient = zd.NewClient(p.ZoomAPIURL, p.APIKey, p.APISecret)

	return nil
}

func (p *Plugin) IsConfigurationValid() error {
	if len(p.APIKey) == 0 {
		return fmt.Errorf("APIKey is not configured.")
	} else if len(p.APISecret) == 0 {
		return fmt.Errorf("APISecret is not configured.")
	} else if len(p.WebhookSecret) == 0 {
		return fmt.Errorf("WebhookSecret is not configured.")
	}

	return nil
}

func (p *Plugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := p.IsConfigurationValid(); err != nil {
		http.Error(w, "This plugin is not configured.", http.StatusNotImplemented)
		return
	}

	switch path := r.URL.Path; path {
	case "/webhook":
		p.handleWebhook(w, r)
	case "/api/v1/meetings":
		p.handleStartMeeting(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("secret")), []byte(p.WebhookSecret)) != 1 {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}

	var webhook zd.Webhook

	decoder := schema.NewDecoder()

	// Try to decode to standard webhook
	if err := decoder.Decode(&webhook, r.PostForm); err == nil {
		p.handleStandardWebhook(w, r, &webhook)
		return
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	// TODO: handle recording webhook
}

func (p *Plugin) handleStandardWebhook(w http.ResponseWriter, r *http.Request, webhook *zd.Webhook) {
	if webhook.Status != zd.WEBHOOK_STATUS_ENDED {
		return
	}

	postId := ""
	key := fmt.Sprintf("%v%v", POST_MEETING_KEY, webhook.ID)
	if b, err := p.API.KVGet(key); err != nil {
		http.Error(w, err.Error(), err.StatusCode)
		return
	} else if b == nil {
		return
	} else {
		postId = string(b)
	}

	post, err := p.API.GetPost(postId)
	if err != nil {
		http.Error(w, err.Error(), err.StatusCode)
		return
	}

	post.Message = "Meeting has ended."
	post.Props["meeting_status"] = zd.WEBHOOK_STATUS_ENDED

	if _, err := p.API.UpdatePost(post); err != nil {
		http.Error(w, err.Error(), err.StatusCode)
		return
	}

	p.API.KVDelete(key)

	w.Write([]byte(post.ToJson()))
}

type StartMeetingRequest struct {
	ChannelId string `json:"channel_id"`
	Personal  bool   `json:"personal"`
	Topic     string `json:"topic"`
	MeetingId int    `json:"meeting_id"`
}

func (p *Plugin) handleStartMeeting(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("Mattermost-User-Id")

	if userId == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req StartMeetingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	var user *model.User
	var err *model.AppError
	user, err = p.API.GetUser(userId)
	if err != nil {
		http.Error(w, err.Error(), err.StatusCode)
	}

	if _, err := p.API.GetChannelMember(req.ChannelId, user.Id); err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	meetingId := req.MeetingId
	personal := req.Personal

	if meetingId == 0 && req.Personal {
		if ru, err := p.zoomClient.GetUser(user.Email); err != nil {
			http.Error(w, err.Error(), err.StatusCode)
			return
		} else {
			meetingId = ru.Pmi
		}
	}

	if meetingId == 0 {
		personal = false

		meeting := &zd.Meeting{
			Type:  zd.MEETING_TYPE_INSTANT,
			Topic: req.Topic,
		}

		if rm, err := p.zoomClient.CreateMeeting(meeting, user.Email); err != nil {
			http.Error(w, err.Error(), err.StatusCode)
			return
		} else {
			meetingId = rm.ID
		}
	}

	zoomUrl := strings.TrimSpace(p.ZoomURL)
	if len(zoomUrl) == 0 {
		zoomUrl = "https://zoom.us"
	}

	meetingUrl := fmt.Sprintf("%s/j/%v", zoomUrl, meetingId)

	post := &model.Post{
		UserId:    user.Id,
		ChannelId: req.ChannelId,
		Message:   fmt.Sprintf("Meeting started at %s.", meetingUrl),
		Type:      "custom_zoom",
		Props: map[string]interface{}{
			"meeting_id":        meetingId,
			"meeting_link":      meetingUrl,
			"meeting_status":    zd.WEBHOOK_STATUS_STARTED,
			"meeting_personal":  personal,
			"meeting_topic":     req.Topic,
			"from_webhook":      "true",
			"override_username": "Zoom",
			"override_icon_url": "https://s3.amazonaws.com/mattermost-plugin-media/Zoom+App.png",
		},
	}

	if post, err := p.API.CreatePost(post); err != nil {
		http.Error(w, err.Error(), err.StatusCode)
		return
	} else {
		err = p.API.KVSet(fmt.Sprintf("%v%v", POST_MEETING_KEY, meetingId), []byte(post.Id))
		if err != nil {
			http.Error(w, err.Error(), err.StatusCode)
			return
		}
	}

	w.Write([]byte(fmt.Sprintf("%v", meetingId)))
}
