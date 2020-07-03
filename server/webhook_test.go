package server

import (
	"context"
	"errors"
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
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	r, err := s.sendToWebhook(context.Background(), mattermost.URL, validPayload)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)

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
	var wErr *WebhookValidationError
	assert.True(t, errors.As(err, &wErr))
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
	var wErr *WebhookValidationError
	assert.True(t, errors.As(err, &wErr))
	assert.NotNil(t, r)
	assert.Equal(t, http.StatusBadRequest, r.StatusCode)

	closeBody(r)
}
