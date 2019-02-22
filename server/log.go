// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-server/mlog"
)

func LogLabels(prNumber int, labels []github.Label) {
	labelStrings := make([]string, len(labels))

	for i, label := range labels {
		labelStrings[i] = "`" + *label.Name + "`"
	}

	mlog.Debug(fmt.Sprintf("PR %d has labels: %v", prNumber, strings.Join(labelStrings, ", ")))
}

func LogInfo(msg string, args ...mlog.Field) {
	mlog.Info(msg, args...)
}

func LogError(msg string, args ...mlog.Field) {
	mlog.Error(msg, args...)
}

func LogCritical(msg string, args ...mlog.Field) {
	mlog.Critical(msg, args...)
	panic(fmt.Sprintf(msg, args...))
}

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
			LogError(fmt.Sprintf("Unable to post to Mattermost webhook: %v", err), mlog.String("err", err.Error()))
		}
	}

	LogError(msg, args...)
}
