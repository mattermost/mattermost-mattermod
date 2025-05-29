package server

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

func (s *Server) handleTranslationPR(ctx context.Context, pr *model.PullRequest) {
	if !s.isTranslationPr(pr) {
		return
	}

	err := s.sendTranslationWebhookMessage(ctx, pr, s.Config.TranslationsMattermostMessage)
	if err != nil {
		mlog.Error("Unable to send message ", mlog.Err(err))
	}
}

func (s *Server) sendTranslationWebhookMessage(ctx context.Context, pr *model.PullRequest, msg string) error {
	prURL := fmt.Sprintf("https://github.com/%v/%v/pull/%v", s.Config.Org, pr.RepoName, pr.Number)
	dataMsg := fmt.Sprintf("#### [%v translations PR %v](%v)\n", pr.RepoName, time.Now().UTC().Format(time.RFC3339), prURL)
	_msg := dataMsg + msg
	mlog.Debug("Sending Mattermost message", mlog.String("message", _msg))
	webhookRequest := &Payload{Username: "Weblate", Text: _msg}
	err := s.sendToWebhook(ctx, s.Config.TranslationsMattermostWebhookURL, webhookRequest)
	if err != nil {
		mlog.Error("Unable to post to Mattermost webhook", mlog.Err(err))
		return err
	}
	return nil
}

func (s *Server) isTranslationPr(pr *model.PullRequest) bool {
	return pr.Username == s.Config.TranslationsBot
}
