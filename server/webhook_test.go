package server

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendToWebhookIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validPayload := &Payload{Username: "mattermod", Text: "test"}
	expectedMattermostPayload := "ok"
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(expectedMattermostPayload))
		if err != nil {
			log.Fatal(err)
		}
	}))
	defer mattermost.Close()

	r, err := s.sendToWebhook(mattermost.URL, validPayload)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)
	data, err := ioutil.ReadAll(r.Body)
	assert.Nil(t, err)
	assert.Equal(t, expectedMattermostPayload, string(data))

	closeBody(r)
}

func TestSendToWebhookUsernameNotSetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	invalidPayload := &Payload{Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	r, err := s.sendToWebhook(mattermost.URL, invalidPayload)
	assert.NotNil(t, err)
	assert.Equal(t, err.(*WebhookValidationError), err)
	assert.NotNil(t, r)
	assert.Equal(t, http.StatusBadRequest, r.StatusCode)

	closeBody(r)
}

func TestSendToWebhookWebhookURLNotSetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validPayload := &Payload{Username: "mattermod", Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	r, err := s.sendToWebhook("", validPayload)
	assert.NotNil(t, err)
	assert.Equal(t, err.(*WebhookValidationError), err)
	assert.NotNil(t, r)
	assert.Equal(t, http.StatusBadRequest, r.StatusCode)

	closeBody(r)
}
