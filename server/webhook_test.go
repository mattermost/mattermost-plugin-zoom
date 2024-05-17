package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"

	"github.com/mattermost/mattermost-plugin-zoom/server/zoom"
)

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

func TestWebhookVerifySignature(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}
	p.setConfiguration(testConfig)

	api.On("GetLicense").Return(nil)
	api.On("KVGet", "post_meeting_123").Return(nil, &model.AppError{StatusCode: 200})
	api.On("LogDebug", "Could not get meeting post from KVStore", "error", "")
	p.SetAPI(api)

	requestBody := `{"payload":{"object": {"id": "123", "uuid": "123"}},"event":"meeting.ended"}`

	ts := "1660149894817"
	h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
	_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
	signature := "v0=" + hex.EncodeToString(h.Sum(nil))

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-zm-signature", signature)
	request.Header.Add("x-zm-request-timestamp", ts)

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := io.ReadAll(w.Result().Body)
	t.Log(string(body))

	require.Equal(t, 200, w.Result().StatusCode)
}

func TestWebhookVerifySignatureInvalid(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}
	p.setConfiguration(testConfig)

	api.On("GetLicense").Return(nil)
	api.On("LogWarn", "Could not verify webhook signature: provided signature does not match")
	p.SetAPI(api)

	requestBody := `{"payload":{"object": {"id": "123"}},"event":"meeting.ended"}`

	ts := "1660149894817"
	signature := "v0=7fe2f9e66d133961eff4746eda161096cebe8d677319d66546281d88ea147190"

	w := httptest.NewRecorder()
	reqBody := io.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-zm-signature", signature)
	request.Header.Add("x-zm-request-timestamp", ts)

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := io.ReadAll(w.Result().Body)
	t.Log(string(body))
}

func TestWebhookHandleTranscriptCompleted(t *testing.T) {
	api := &plugintest.API{}
	p := Plugin{}
	p.setConfiguration(testConfig)

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer httpServer.Close()

	oldDefaultClient := http.DefaultClient
	http.DefaultClient = httpServer.Client()
	defer func() {
		http.DefaultClient = oldDefaultClient
	}()

	api.On("GetLicense").Return(nil)
	api.On("GetPost", "post-id").Return(&model.Post{Id: "post-id", ChannelId: "channel-id"}, nil)
	api.On("KVGet", "post_meeting_321").Return([]byte("post-id"), nil)
	api.On("UploadFile", []byte("/test"), "channel-id", "transcription.txt").Return(&model.FileInfo{Id: "file-id"}, nil)
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

	ts := "1660149894817"
	h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
	_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
	signature := "v0=" + hex.EncodeToString(h.Sum(nil))

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
	p.setConfiguration(testConfig)

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer httpServer.Close()

	oldDefaultClient := http.DefaultClient
	http.DefaultClient = httpServer.Client()
	defer func() {
		http.DefaultClient = oldDefaultClient
	}()

	api.On("GetLicense").Return(nil)
	api.On("GetPost", "post-id").Return(&model.Post{Id: "post-id", ChannelId: "channel-id"}, nil)
	api.On("KVGet", "post_meeting_321").Return([]byte("post-id"), nil)
	api.On("UploadFile", []byte("/chat_file"), "channel-id", "Chat-history.txt").Return(&model.FileInfo{Id: "file-id"}, nil)
	api.On("CreatePost", &model.Post{
		ChannelId: "channel-id",
		RootId:    "post-id",
		Message:   "Here's the zoom meeting recording:\n**Link:** [Meeting Recording]()\n**Password:** test-password",
		Type:      "custom_zoom_chat",
		Props: model.StringInterface{
			"captions": []any{map[string]any{"file_id": "file-id"}},
		},
		FileIds: []string{"file-id"},
	}).Return(&model.Post{
		ChannelId: "channel-id",
		RootId:    "post-id",
		Message:   "Here's the zoom meeting recording:\n**Link:** [Meeting Recording]()\n**Password:** test-password",
		Type:      "custom_zoom_chat",
		Props: model.StringInterface{
			"captions": []any{map[string]any{"file_id": "file-id"}},
		},
		FileIds: []string{"file-id"},
	}, nil)
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
						"recording_type":  "shared_screen_with_speaker_view",
						"download_url":    httpServer.URL + "/recording_file",
						"playURL":         httpServer.URL + "/recording_url",
					},
				},
			},
		},
		"event":          "recording.completed",
		"download_token": "test-token",
	})
	requestBody := string(requestBodyBytes)

	ts := "1660149894817"
	h := hmac.New(sha256.New, []byte(testConfig.ZoomWebhookSecret))
	_, _ = h.Write([]byte("v0:" + ts + ":" + requestBody))
	signature := "v0=" + hex.EncodeToString(h.Sum(nil))

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
