// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type WebhookValidationError struct {
	err string
}

func (e *WebhookValidationError) Error() string {
	return fmt.Sprintf("%v", e.err)
}

type Payload struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

func (s *Server) sendToWebhook(webhookURL string, payload *Payload) (*http.Response, error) {
	err := validateSendToWebhookRequest(webhookURL, payload)
	if err != nil {
		badRequestR := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       ioutil.NopCloser(bytes.NewBufferString(err.Error())),
		}
		return badRequestR, err
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		internalServerErrorR := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       ioutil.NopCloser(bytes.NewBufferString(err.Error())),
		}
		return internalServerErrorR, err
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest(http.MethodPost, webhookURL, body)
	if err != nil {
		internalServerErrorR := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       ioutil.NopCloser(bytes.NewBufferString(err.Error())),
		}
		return internalServerErrorR, err
	}
	req.Header.Set("Content-Type", "application/json")

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return r, err
	}

	return r, nil
}

func validateSendToWebhookRequest(webhookURL string, payload *Payload) error {
	if webhookURL == "" {
		return &WebhookValidationError{"webook url not set"}
	}

	if payload.Username == "" {
		return &WebhookValidationError{"username not set"}
	}

	if payload.Text == "" {
		return &WebhookValidationError{"text not set"}
	}
	return nil
}
