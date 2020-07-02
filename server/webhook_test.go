package server

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestSendToWebhookIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	s := &Server{
		StartTime: time.Now(),
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

	r, err := s.sendToWebhook(context.Background(), mattermost.URL, validPayload)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)
	data, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	assert.Equal(t, expectedMattermostPayload, string(data))

	closeBody(r)
}

func TestSendToWebhookUsernameNotSetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	s := &Server{
		StartTime: time.Now(),
	}

	invalidPayload := &Payload{Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	r, err := s.sendToWebhook(context.Background(), mattermost.URL, invalidPayload)
	require.Error(t, err.(*WebhookValidationError), err)
	assert.NotNil(t, r)
	assert.Equal(t, http.StatusBadRequest, r.StatusCode)

	closeBody(r)
}

func TestSendToWebhookWebhookURLNotSetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	s := &Server{
		StartTime: time.Now(),
	}

	validPayload := &Payload{Username: "mattermod", Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	r, err := s.sendToWebhook(context.Background(), "", validPayload)
	require.Error(t, err.(*WebhookValidationError), err)
	assert.NotNil(t, r)
	assert.Equal(t, http.StatusBadRequest, r.StatusCode)

	closeBody(r)
}
