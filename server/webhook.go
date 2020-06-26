// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

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
		return errors.New("no Mattermost webhook URL set: unable to send message")
	}

	if payload.Username == "" {
		return errors.New("username not set in webhook payload")
	}

	if payload.Text == "" {
		return errors.New("text not set in webhook payload")
	}
	return nil
}
