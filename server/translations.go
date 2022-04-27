package server

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

func (s *Server) handleTranslationPR(ctx context.Context, pr *model.PullRequest) {
	if pr.Username != s.Config.TranslationsBot {
		return
	}

	prURL := fmt.Sprintf("https://github.com/%v/%v/pull/%v", s.Config.Org, pr.RepoName, pr.Number)
	dataMsg := fmt.Sprintf("#### [%v translations PR %v](%v)\n", pr.RepoName, time.Now().UTC().Format(time.RFC3339), prURL)
	msg := dataMsg + s.Config.TranslationsMattermostMessage
	mlog.Debug("Sending Mattermost message", mlog.String("message", msg))

	webhookRequest := &Payload{Username: "Weblate", Text: msg}
	err := s.sendToWebhook(ctx, s.Config.TranslationsMattermostWebhookURL, webhookRequest)
	if err != nil {
		mlog.Error("Unable to post to Mattermost webhook", mlog.Err(err))
		return
	}
}
