package server

import (
	"fmt"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) logToMattermost(msg string, args ...interface{}) {
	webhookMessage := fmt.Sprintf(msg, args...)
	mlog.Debug("Sending Mattermost message", mlog.String("message", webhookMessage))

	if s.Config.MattermostWebhookFooter != "" {
		webhookMessage += "\n---\n" + s.Config.MattermostWebhookFooter
	}

	webhookRequest := &Payload{Username: "Mattermod", Text: webhookMessage}

	if err := s.sendToWebhook(s.Config.MattermostWebhookURL, webhookRequest); err != nil {
		mlog.Error("Unable to post to Mattermost webhook", mlog.Err(err))
	}
}

func NewBool(b bool) *bool       { return &b }
func NewInt(n int) *int          { return &n }
func NewInt64(n int64) *int64    { return &n }
func NewInt32(n int32) *int32    { return &n }
func NewString(s string) *string { return &s }

func contains(list []string, item string) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}
