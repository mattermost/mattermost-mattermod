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

type WebhookRequest struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

func (s *Server) sendToWebhook(webhookRequest *WebhookRequest, url string) error {
	b, err := json.Marshal(webhookRequest)
	if err != nil {
		return err
	}

	client := http.Client{}
	request, err := http.NewRequest("POST", s.Config.MattermostWebhookURL, bytes.NewReader(b))
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		contents, _ := ioutil.ReadAll(response.Body)
		return errors.Errorf("received non-200 status code posting to mattermost: %v %v", contents, response.StatusCode)
	}

	return nil
}
