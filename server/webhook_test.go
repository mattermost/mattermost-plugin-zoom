package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"

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
	reqBody := ioutil.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := ioutil.ReadAll(w.Result().Body)
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
	api.On("LogDebug", "Could not get meeting post from KVStore", "err", ": , ")
	p.SetAPI(api)

	requestBody := `{"payload":{"object": {"id": "123"}},"event":"meeting.ended"}`

	ts := "1660149894817"
	signature := "v0=7fe2f9e66d133961eff4746eda161096cebe8d677319d66546281d88ea147189"

	w := httptest.NewRecorder()
	reqBody := ioutil.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-zm-signature", signature)
	request.Header.Add("x-zm-request-timestamp", ts)

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := ioutil.ReadAll(w.Result().Body)
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
	reqBody := ioutil.NopCloser(bytes.NewBufferString(requestBody))
	request := httptest.NewRequest("POST", "/webhook?secret=webhooksecret", reqBody)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-zm-signature", signature)
	request.Header.Add("x-zm-request-timestamp", ts)

	p.ServeHTTP(&plugin.Context{}, w, request)
	body, _ := ioutil.ReadAll(w.Result().Body)
	t.Log(string(body))
}
