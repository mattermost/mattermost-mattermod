// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/mlog"
)

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

	// Safely convert args into serializable fields
	var builder strings.Builder
	for _, arg := range args {
		builder.WriteString(fmt.Sprintf("%v", arg))
	}
	mlog.Error(fmt.Sprintf("%v %v", msg, builder.String()))
}
