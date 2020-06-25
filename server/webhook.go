// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/mlog"

	"github.com/pkg/errors"
)

type Payload struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

func (s *Server) sendToWebhook(webhookURL string, payload *Payload) error {
	if webhookURL == "" {
		err := errors.New("no Mattermost webhook URL set: unable to send message")
		mlog.Err(err)
		return err
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest(http.MethodPost, webhookURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return errors.Errorf("received non-200 status code posting to mattermost: %v, %v", r.StatusCode, r.Body)
	}

	return nil
}
