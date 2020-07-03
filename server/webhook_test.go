package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestSendToWebhookIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	s := &Server{}

	validPayload := &Payload{Username: "mattermod", Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	err := s.sendToWebhook(context.Background(), mattermost.URL, validPayload)
	require.NoError(t, err)
}

func TestSendToWebhookUsernameNotSetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	s := &Server{}

	invalidPayload := &Payload{Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	err := s.sendToWebhook(context.Background(), mattermost.URL, invalidPayload)
	var whError *WebhookValidationError
	require.True(t, errors.As(err, &whError))
	assert.Equal(t, whError.field, "username")
}

func TestSendToWebhookWebhookURLNotSetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	s := &Server{}

	validPayload := &Payload{Username: "mattermod", Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	err := s.sendToWebhook(context.Background(), "", validPayload)
	var whError *WebhookValidationError
	require.True(t, errors.As(err, &whError))
	assert.Equal(t, whError.field, "webhook URL")
}
