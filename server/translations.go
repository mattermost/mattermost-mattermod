package server

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) handleTranslationPR(ctx context.Context, pr *model.PullRequest) {
	if pr.Username != s.Config.TranslationsBot {
		return
	}

	dataMsg := fmt.Sprintf("####[%v translations PR %v](%v)\n", pr.RepoName, time.Now().UTC().Format(time.RFC3339), pr.URL)
	msg := dataMsg + s.Config.TranslationsMattermostMessage
	mlog.Debug("Sending Mattermost message", mlog.String("message", msg))

	webhookRequest := &Payload{Username: "Weblate", Text: msg}
	err := s.sendToWebhook(ctx, s.Config.TranslationsMattermostWebhookURL, webhookRequest)
	if err != nil {
		mlog.Error("Unable to post to Mattermost webhook", mlog.Err(err))
		return
	}
}

func (s *Server) handleModificationOfLanguageFiles(ctx context.Context, pr *model.PullRequest) {
	repoConfig, isTranslated := isTranslatedRepo(pr, s.Config.TranslationsRepos)
	if !isTranslated {
		return
	}

	if s.Config.TranslationsBot == pr.Username {
		return
	}

	prFiles, err := s.getPRFiles(ctx, pr.RepoOwner, pr.RepoName, pr.Number)
	if err != nil {
		mlog.Error("Error listing PR files", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.Err(err))
		return
	}

	languageDir, languageBaseFile := path.Split(repoConfig.BaseFilePath)

	if !isModifyingLanguageDir(prFiles, languageDir) {
		return
	}

	if isOnlyModifyingAllowedFiles(prFiles, languageDir, languageBaseFile) {
		return
	}

	mlog.Info("Modification of language file", mlog.String("repo", pr.RepoName), mlog.Int("pr", pr.Number), mlog.String("username", pr.Username))
	msg := fmt.Sprintf("Thank you for your contribution, @%s! For translating, please use [https://translate.mattermost.com](https://translate.mattermost.com) and join our community on [https://community.mattermost.com/core/channels/localization](https://community.mattermost.com/core/channels/localization) ", pr.Username)
	s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg)

	_, _, err = s.GithubClient.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, []string{"Lifecycle/1:stale", "Do Not Merge"})
	if err != nil {
		msg := fmt.Sprintf("error applying labels %v:\n ```%v```", pr.URL, err.Error())
		s.logToMattermost(ctx, msg)
		mlog.Error("Error applying label", mlog.Err(err), mlog.Int("PR", pr.Number), mlog.String("Repo", pr.RepoName))
		return
	}
}

func isTranslatedRepo(pr *model.PullRequest, translationsRepo []*TranslationsRepo) (*TranslationsRepo, bool) {
	for _, t := range translationsRepo {
		if t.RepoName == pr.RepoName {
			return t, true
		}
	}
	return nil, false
}

func isModifyingLanguageDir(prFiles []*github.CommitFile, languageDir string) bool {
	for _, prFile := range prFiles {
		prFilePath, _ := path.Split(prFile.GetFilename())
		if prFilePath == languageDir {
			return true
		}
	}
	return false
}

func isOnlyModifyingAllowedFiles(prFiles []*github.CommitFile, languageDir string, languageBaseFile string) bool {
	for _, prFileName := range getPRFileNamesModifyingLanguageDir(prFiles, languageDir) {
		if prFileName == languageBaseFile || filepath.Ext(prFileName) == "json" {
			continue
		} else {
			return false
		}
	}
	return true
}

func getPRFileNamesModifyingLanguageDir(prFiles []*github.CommitFile, languageDir string) []string {
	var prFilesModifyingLanguageDir []string
	for _, prFile := range prFiles {
		prFilePath, prFileName := path.Split(prFile.GetFilename())
		if prFilePath == languageDir {
			prFilesModifyingLanguageDir = append(prFilesModifyingLanguageDir, prFileName)
		}
	}
	return prFilesModifyingLanguageDir
}
