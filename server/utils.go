package server

import (
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
)

func (s *Server) logErrorToMattermost(msg string, args ...interface{}) {
	if s.Config.MattermostWebhookURL == "" {
		mlog.Warn("No Mattermost webhook URL set: unable to send message")
		return
	}

	webhookMessage := fmt.Sprintf(msg, args...)
	mlog.Debug("Sending Mattermost message", mlog.String("message", webhookMessage))

	if s.Config.MattermostWebhookFooter != "" {
		webhookMessage += "\n---\n" + s.Config.MattermostWebhookFooter
	}

	webhookRequest := &WebhookRequest{Username: "Mattermod", Text: webhookMessage}

	if err := s.sendToWebhook(webhookRequest, s.Config.MattermostWebhookURL); err != nil {
		mlog.Error("Unable to post to Mattermost webhook", mlog.Err(err))
	}
}

func (s *Server) logPrettyErrorToMattermost(msg string, pr *model.PullRequest, err error, additionalFields map[string]string) {
	if s.Config.MattermostWebhookURL == "" {
		mlog.Warn("No Mattermost webhook URL set: unable to send message")
		return
	}

	mlog.Debug("Sending Mattermost message", mlog.String("message", msg))

	fullMessage := fmt.Sprintf("%s\n---\nError: %s\nRepository: %s/%s\nPull Request: %d [ status=%s ]\nURL: %s\n",
		msg,
		err,
		pr.RepoOwner, pr.RepoName,
		pr.Number, pr.State,
		pr.URL,
	)
	for key, value := range additionalFields {
		fullMessage = fullMessage + fmt.Sprintf("%s: %s\n", key, value)
	}
	fullMessage = fullMessage + s.Config.MattermostWebhookFooter

	webhookRequest := &WebhookRequest{Username: "Mattermod", Text: fullMessage}

	if err := s.sendToWebhook(webhookRequest, s.Config.MattermostWebhookURL); err != nil {
		mlog.Error("Unable to post to Mattermost webhook", mlog.Err(err))
	}
}

func NewBool(b bool) *bool       { return &b }
func NewInt(n int) *int          { return &n }
func NewInt64(n int64) *int64    { return &n }
func NewInt32(n int32) *int32    { return &n }
func NewString(s string) *string { return &s }
