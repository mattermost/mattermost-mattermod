// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"log"
	"os"

	"github.com/mattermost/mattermost-server/mlog"
)

func Log(level string, msg string, args ...interface{}) {
	log.Printf("%v %v\n", level, fmt.Sprintf(msg, args...))
	f, err := os.OpenFile("mattermod.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Failed to write to file")
		return
	}
	defer f.Close()

	log.SetOutput(f)
	log.Printf("%v %v\n", level, fmt.Sprintf(msg, args...))
}

func LogErrorToMattermost(msg string, args ...interface{}) {
	if Config.MattermostWebhookURL != "" {
		webhookMessage := fmt.Sprintf(msg, args...)
		if Config.MattermostWebhookFooter != "" {
			webhookMessage += "\n---\n" + Config.MattermostWebhookFooter
		}

		webhookRequest := &WebhookRequest{Username: "Mattermod", Text: webhookMessage}

		if err := sendToWebhook(webhookRequest, Config.MattermostWebhookURL); err != nil {
			mlog.Error(fmt.Sprintf("Unable to post to Mattermost webhook: %v", err.Error()))
		}
	}

	mlog.Error(msg, args...)
}
