package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendToWebhookIntegration(t *testing.T) {
	s := &Server{}

	validPayload := &Payload{Username: "mattermod", Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p Payload
		err := json.NewDecoder(r.Body).Decode(&p)
		require.NoError(t, err)
		assert.Equal(t, validPayload.Username, p.Username)
		assert.Equal(t, validPayload.Text, p.Text)
	}))
	defer mattermost.Close()

	err := s.sendToWebhook(context.Background(), mattermost.URL, validPayload)
	require.NoError(t, err)
}

func TestSendToWebhookIntegrationInvalid(t *testing.T) {
	s := &Server{}

	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	t.Run("UsernameNotSet", func(t *testing.T) {
		invalid := &Payload{Text: "test"}
		err := s.sendToWebhook(context.Background(), mattermost.URL, invalid)
		var whError *WebhookValidationError
		require.True(t, errors.As(err, &whError))
		assert.Equal(t, whError.field, "username")
	})

	t.Run("TextNotSet", func(t *testing.T) {
		invalid := &Payload{Username: "mattermod"}
		err := s.sendToWebhook(context.Background(), mattermost.URL, invalid)
		var whError *WebhookValidationError
		require.True(t, errors.As(err, &whError))
		assert.Equal(t, whError.field, "text")
	})

	t.Run("URLNotSet", func(t *testing.T) {
		valid := &Payload{Username: "mattermod", Text: "test"}
		err := s.sendToWebhook(context.Background(), "", valid)
		var whError *WebhookValidationError
		require.True(t, errors.As(err, &whError))
		assert.Equal(t, whError.field, "webhook URL")
	})
}
