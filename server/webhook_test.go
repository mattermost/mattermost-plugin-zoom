// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/telemetry"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

func allowFlexibleLogging(api *plugintest.API) {
	for _, method := range []string{"LogDebug", "LogWarn", "LogError"} {
		api.On(method, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
		api.On(method, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
		api.On(method, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
		api.On(method, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
		api.On(method, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
		api.On(method, mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
		api.On(method, mock.Anything).Maybe().Return()
	}
}

var testConfig = &configuration{
	OAuthClientID:     "clientid",
	OAuthClientSecret: "clientsecret",
	EncryptionKey:     "encryptionkey",
	WebhookSecret:     "webhooksecret",
	ZoomWebhookSecret: "zoomwebhooksecret",
}

func TestWebhookValidate(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}
	p.setConfiguration(testConfig)

	api.On("GetLicense").Return(nil)
	allowFlexibleLogging(api)
	p.SetAPI(api)

	requestBody := `{"payload":{"plainToken":"Kn5a3Wv7SP6YP5b4BWfZpg"},"event":"endpoint.url_validation"}`

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := io.ReadAll(w.Result().Body)
	t.Log(string(body))

	require.Equal(t, 200, w.Result().StatusCode)

	out := zoom.ValidationWebhookResponse{}
	err := json.Unmarshal(body, &out)
	require.NoError(t, err)

	require.Equal(t, "Kn5a3Wv7SP6YP5b4BWfZpg", out.PlainToken)
	require.Equal(t, "2a41c3138d2187a756c51428f78d192e9b88dcf44dd62d1b081ace4ec2241e0a", out.EncryptedToken)
}

func TestHandleMeetingStarted(t *testing.T) {
	p := Plugin{}
	p.setConfiguration(testConfig)

	t.Run("successful meeting start", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetLicense").Return(nil)
		api.On("KVGet", "meeting_channel_123").Return([]byte("channel-id"), nil)
		api.On("GetUser", "").Return(&model.User{Id: "user-id"}, nil)
		api.On("KVGet", "zoomtoken_user-id").Return(nil, &model.AppError{})
		api.On("LogWarn", "could not get the active Zoom client", "error", "could not fetch Zoom OAuth info: must connect user account to Zoom first").Return()
		api.On("HasPermissionToChannel", "user-id", "channel-id", mock.AnythingOfType("*model.Permission")).Return(true)
		api.On("KVSetWithExpiry", "post_meeting_abc", []byte{}, int64(86400)).Return(nil)
		api.On("KVSetWithExpiry", "meeting_channel_123", mock.AnythingOfType("[]uint8"), int64(adHocMeetingChannelTTL)).Return(nil)
		api.On("PublishWebSocketEvent", "meeting_started", map[string]interface{}{"meeting_url": "https://zoom.us/j/123"}, mock.AnythingOfType("*model.WebsocketBroadcast")).Return()
		api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)
		api.On("GetPostsSince", "channel-id", mock.AnythingOfType("int64")).Return(&model.PostList{}, nil)
		allowFlexibleLogging(api)
		p.SetAPI(api)
		p.client = pluginapi.NewClient(api, nil)
		p.botUserID = ""
		p.tracker = telemetry.NewTracker(nil, "", "", "", "", "", telemetry.NewTrackerConfig(nil), nil)

		requestBody := `{"payload":{"object": {"id": "123", "uuid": "abc", "topic": "test meeting"}},"event":"meeting.started"}`
		w := httptest.NewRecorder()
		reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
		request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
		request.Header.Add("Content-Type", "application/json")

		ts := fmt.Sprintf("%d", time.Now().Unix())
		h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
		_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
		signature := "v0=" + hex.EncodeToString(h.Sum(nil))

		request.Header.Add("x-zm-signature", signature)
		request.Header.Add("x-zm-request-timestamp", ts)

		p.ServeHTTP(&plugin.Context{}, w, request)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	})

	t.Run("invalid meeting ID", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetLicense").Return(nil)
		api.On("LogError", "Failed to get meeting ID", "err", "strconv.Atoi: parsing \"invalid\": invalid syntax").Return()
		allowFlexibleLogging(api)
		p.SetAPI(api)

		requestBody := `{"payload":{"object": {"id": "invalid", "uuid": "123-abc"}},"event":"meeting.started"}`
		w := httptest.NewRecorder()
		reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
		request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
		request.Header.Add("Content-Type", "application/json")

		ts := fmt.Sprintf("%d", time.Now().Unix())
		h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
		_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
		signature := "v0=" + hex.EncodeToString(h.Sum(nil))

		request.Header.Add("x-zm-signature", signature)
		request.Header.Add("x-zm-request-timestamp", ts)

		p.ServeHTTP(&plugin.Context{}, w, request)
		require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	})

	t.Run("channel not found", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetLicense").Return(nil)
		api.On("KVGet", "meeting_channel_123").Return(nil, &model.AppError{})
		api.On("KVSetWithExpiry", "post_meeting_123-abc", []byte{}, int64(86400)).Return(nil)
		allowFlexibleLogging(api)
		p.SetAPI(api)

		requestBody := `{"payload":{"object": {"id": "123", "uuid": "123-abc"}},"event":"meeting.started"}`
		w := httptest.NewRecorder()
		reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
		request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
		request.Header.Add("Content-Type", "application/json")

		ts := fmt.Sprintf("%d", time.Now().Unix())
		h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
		_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
		signature := "v0=" + hex.EncodeToString(h.Sum(nil))

		request.Header.Add("x-zm-signature", signature)
		request.Header.Add("x-zm-request-timestamp", ts)

		p.ServeHTTP(&plugin.Context{}, w, request)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	})
}

func TestWebhookVerifySignature(t *testing.T) {
	t.Run("recent timestamp with valid signature", func(t *testing.T) {
		api := &plugintest.API{}
		p := Plugin{}
		p.setConfiguration(testConfig)

		api.On("GetLicense").Return(nil)
		api.On("KVGet", "post_meeting_123-abc").Return(nil, &model.AppError{StatusCode: 200})
		api.On("KVGet", "meeting_channel_123").Return(nil, (*model.AppError)(nil))
		allowFlexibleLogging(api)
		p.SetAPI(api)
		p.client = pluginapi.NewClient(p.API, p.Driver)

		requestBody := `{"payload":{"object": {"id": "123", "uuid": "123-abc"}},"event":"meeting.ended"}`

		ts := fmt.Sprintf("%d", time.Now().Unix())
		msg := fmt.Sprintf("v0:%s:%s", ts, requestBody)
		hash, _ := createWebhookSignatureHash("zoomwebhooksecret", msg)
		signature := fmt.Sprintf("v0=%s", hash)

		w := httptest.NewRecorder()
		reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
		request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
		request.Header.Add("Content-Type", "application/json")
		request.Header.Add("x-zm-signature", signature)
		request.Header.Add("x-zm-request-timestamp", ts)

		p.ServeHTTP(&plugin.Context{}, w, request)
		body, _ := io.ReadAll(w.Result().Body)
		t.Log(string(body))

		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("old timestamp is rejected", func(t *testing.T) {
		api := &plugintest.API{}
		p := Plugin{}
		p.setConfiguration(testConfig)

		api.On("GetLicense").Return(nil)
		api.On("LogWarn", "Could not verify webhook signature: webhook timestamp is too old")
		allowFlexibleLogging(api)
		p.SetAPI(api)

		requestBody := `{"payload":{"object": {"id": "123"}},"event":"meeting.ended"}`

		ts := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
		msg := fmt.Sprintf("v0:%s:%s", ts, requestBody)
		hash, _ := createWebhookSignatureHash("zoomwebhooksecret", msg)
		signature := fmt.Sprintf("v0=%s", hash)

		w := httptest.NewRecorder()
		reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
		request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
		request.Header.Add("Content-Type", "application/json")
		request.Header.Add("x-zm-signature", signature)
		request.Header.Add("x-zm-request-timestamp", ts)

		p.ServeHTTP(&plugin.Context{}, w, request)

		require.Equal(t, 401, w.Result().StatusCode)
	})

	t.Run("future timestamp is rejected", func(t *testing.T) {
		api := &plugintest.API{}
		p := Plugin{}
		p.setConfiguration(testConfig)

		api.On("GetLicense").Return(nil)
		api.On("LogWarn", "Could not verify webhook signature: webhook timestamp is too far in the future")
		allowFlexibleLogging(api)
		p.SetAPI(api)

		requestBody := `{"payload":{"object": {"id": "123"}},"event":"meeting.ended"}`

		ts := fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix())
		msg := fmt.Sprintf("v0:%s:%s", ts, requestBody)
		hash, _ := createWebhookSignatureHash("zoomwebhooksecret", msg)
		signature := fmt.Sprintf("v0=%s", hash)

		w := httptest.NewRecorder()
		reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
		request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
		request.Header.Add("Content-Type", "application/json")
		request.Header.Add("x-zm-signature", signature)
		request.Header.Add("x-zm-request-timestamp", ts)

		p.ServeHTTP(&plugin.Context{}, w, request)

		require.Equal(t, 401, w.Result().StatusCode)
	})
}

func TestWebhookEmptyZoomWebhookSecret(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}

	configWithoutSecret := &configuration{
		OAuthClientID:     "clientid",
		OAuthClientSecret: "clientsecret",
		EncryptionKey:     "encryptionkey",
		WebhookSecret:     "webhooksecret",
		ZoomWebhookSecret: "",
	}
	p.setConfiguration(configWithoutSecret)

	api.On("GetLicense").Return(nil)
	api.On("KVGet", "post_meeting_123").Return(nil, &model.AppError{StatusCode: 200})
	api.On("KVGet", "meeting_channel_123").Return(nil, (*model.AppError)(nil))
	allowFlexibleLogging(api)
	p.SetAPI(api)
	p.client = pluginapi.NewClient(p.API, p.Driver)

	requestBody := `{"payload":{"object": {"id": "123", "uuid": "123"}},"event":"meeting.ended"}`

	ts := "1660149894817"
	h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
	_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")

	p.ServeHTTP(&plugin.Context{}, w, request)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestWebhookVerifySignatureInvalid(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}
	p.setConfiguration(testConfig)

	api.On("GetLicense").Return(nil)
	api.On("LogWarn", "Could not verify webhook signature: provided signature does not match")
	allowFlexibleLogging(api)
	p.SetAPI(api)

	requestBody := `{"payload":{"object": {"id": "123"}},"event":"meeting.ended"}`

	ts := fmt.Sprintf("%d", time.Now().Unix())
	signature := "v0=invalidsignature1234567890abcdef1234567890abcdef1234567890abcd"

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-zm-signature", signature)
	request.Header.Add("x-zm-request-timestamp", ts)

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := io.ReadAll(w.Result().Body)
	t.Log(string(body))

	require.Equal(t, 401, w.Result().StatusCode)
}

func TestWebhookBodyTooLarge(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}
	p.setConfiguration(testConfig)

	api.On("GetLicense").Return(nil)
	api.On("LogWarn", "Webhook request body too large")
	p.SetAPI(api)

	largeBody := make([]byte, maxWebhookBodySize+100)
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewReader(largeBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")

	p.ServeHTTP(&plugin.Context{}, w, request)

	require.Equal(t, 413, w.Result().StatusCode)
}

func TestWebhookHandleTranscriptCompleted(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}

	httpServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer httpServer.Close()

	cfg := *testConfig
	cfg.ZoomURL = httpServer.URL
	p.setConfiguration(&cfg)

	oldDefaultClient := http.DefaultClient
	http.DefaultClient = httpServer.Client()
	defer func() {
		http.DefaultClient = oldDefaultClient
	}()

	api.On("GetLicense").Return(nil)
	api.On("GetPost", "post-id").Return(&model.Post{Id: "post-id", ChannelId: "channel-id"}, nil)
	api.On("KVGet", "post_meeting_321").Return([]byte("post-id"), nil)
	allowFlexibleLogging(api)
	api.On("UploadFile", []byte("/test"), "channel-id", "transcription.txt").Return(&model.FileInfo{Id: "file-id"}, nil)
	p.client = pluginapi.NewClient(api, nil)
	api.On("CreatePost", &model.Post{
		ChannelId: "channel-id",
		RootId:    "post-id",
		Message:   "Here's the zoom meeting transcription",
		Type:      "custom_zoom_transcript",
		Props: model.StringInterface{
			"captions": []any{map[string]any{"file_id": "file-id"}},
		},
		FileIds: []string{"file-id"},
	}).Return(&model.Post{
		ChannelId: "channel-id",
		RootId:    "post-id",
		Message:   "Here's the zoom meeting transcription",
		Type:      "custom_zoom_transcript",
		Props: model.StringInterface{
			"captions": []any{map[string]any{"file_id": "file-id"}},
		},
		FileIds: []string{"file-id"},
	}, nil)
	p.SetAPI(api)

	requestBodyBytes, _ := json.Marshal(map[string]any{
		"payload": map[string]any{
			"object": map[string]any{
				"id":   123,
				"uuid": "321",
				"recording_files": []map[string]any{
					{
						"recording_type": "audio_transcript",
						"download_url":   httpServer.URL + "/test",
					},
				},
			},
		},
		"event":          "recording.transcript_completed",
		"download_token": "test-token",
	})
	requestBody := string(requestBodyBytes)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
	_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
	signature := fmt.Sprintf("v0=%s", hex.EncodeToString(h.Sum(nil)))

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-zm-signature", signature)
	request.Header.Add("x-zm-request-timestamp", ts)

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := io.ReadAll(w.Result().Body)
	t.Log(string(body))

	api.AssertExpectations(t)
}

func TestWebhookHandleRecordingCompleted(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}

	httpServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer httpServer.Close()

	cfg := *testConfig
	cfg.ZoomURL = httpServer.URL
	p.setConfiguration(&cfg)

	oldDefaultClient := http.DefaultClient
	http.DefaultClient = httpServer.Client()
	defer func() {
		http.DefaultClient = oldDefaultClient
	}()

	api.On("GetLicense").Return(nil)
	api.On("GetPost", "post-id").Return(&model.Post{Id: "post-id", ChannelId: "channel-id"}, nil)
	api.On("KVGet", "post_meeting_321").Return([]byte("post-id"), nil)
	allowFlexibleLogging(api)
	api.On("UploadFile", []byte("/chat_file"), "channel-id", "Chat-history.txt").Return(&model.FileInfo{Id: "file-id"}, nil)
	p.client = pluginapi.NewClient(api, nil)
	api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)
	p.SetAPI(api)

	now := time.Now()
	requestBodyBytes, _ := json.Marshal(map[string]any{
		"payload": map[string]any{
			"object": map[string]any{
				"id":       123,
				"uuid":     "321",
				"password": "test-password",
				"recording_files": []map[string]any{
					{
						"recording_start": now,
						"recording_type":  "chat_file",
						"download_url":    httpServer.URL + "/chat_file",
					},
					{
						"recording_start": now,
						"recording_type":  "shared_screen_with_speaker_view(CC)",
						"file_type":       "MP4",
						"download_url":    httpServer.URL + "/recording_file",
						"play_url":        httpServer.URL + "/recording_url",
					},
				},
			},
		},
		"event":          "recording.completed",
		"download_token": "test-token",
	})
	requestBody := string(requestBodyBytes)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
	_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
	signature := fmt.Sprintf("v0=%s", hex.EncodeToString(h.Sum(nil)))

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-zm-signature", signature)
	request.Header.Add("x-zm-request-timestamp", ts)

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := io.ReadAll(w.Result().Body)
	t.Log(string(body))

	api.AssertExpectations(t)
}
