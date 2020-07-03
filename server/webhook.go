// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// WebhookValidationError contains an error in the webhook payload.
type WebhookValidationError struct {
	field string
}

// Error implements the error interface.
func (e *WebhookValidationError) Error() string {
	return "invalid" + e.field
}

func newWebhookValidationError(field string) *WebhookValidationError {
	return &WebhookValidationError{
		field: field,
	}
}

type Payload struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

func (s *Server) sendToWebhook(ctx context.Context, webhookURL string, payload *Payload) error {
	err := validateSendToWebhookRequest(webhookURL, payload)
	if err != nil {
		return err
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	closeBody(r)

	return nil
}

func validateSendToWebhookRequest(webhookURL string, payload *Payload) error {
	if webhookURL == "" {
		return newWebhookValidationError("webhook URL")
	}

	if payload.Username == "" {
		return newWebhookValidationError("username")
	}

	if payload.Text == "" {
		return &WebhookValidationError{"text"}
	}
	return nil
}
