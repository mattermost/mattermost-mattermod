package server

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleTranslationPr(pr *model.PullRequest) {
	if pr.Username != s.Config.Username {
		return
	}

	dataMsg := fmt.Sprintf("####[%v translations PR %v](%v)\n", pr.RepoName, time.Now().UTC().Format(time.RFC3339), pr.URL)
	msg := dataMsg + s.Config.TranslationsMattermostMessage
	mlog.Debug("Sending Mattermost message", mlog.String("message", msg))

	webhookRequest := &Payload{Username: "Weblate", Text: msg}
	r, err := s.sendToWebhook(s.Config.TranslationsMattermostWebhookURL, webhookRequest)
	if err != nil {
		mlog.Error("Unable to post to Mattermost webhook", mlog.Err(err))
		return
	}
	defer r.Body.Close()
}
