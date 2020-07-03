package server

import (
	"testing"

	"github.com/google/go-github/v32/github"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/stretchr/testify/assert"
)

func TestIsTranslatedRepo(t *testing.T) {
	pr := &model.PullRequest{RepoName: "mattermost-server"}
	expectedTranslationsRepo := &TranslationsRepo{
		RepoName: "mattermost-server",
	}
	translationsRepos := []*TranslationsRepo{expectedTranslationsRepo}
	translationsRepo, isTranslatedRepo := isTranslatedRepo(pr, translationsRepos)
	assert.True(t, isTranslatedRepo)
	assert.EqualValues(t, expectedTranslationsRepo, translationsRepo)
}

func TestIsNotTranslatedRepo(t *testing.T) {
	pr := &model.PullRequest{RepoName: "mattermost-server"}
	translationsRepo := &TranslationsRepo{
		RepoName: "mattermost",
	}
	translationsRepos := []*TranslationsRepo{translationsRepo}
	translationsRepo, isTranslatedRepo := isTranslatedRepo(pr, translationsRepos)
	assert.False(t, isTranslatedRepo)
	assert.Nil(t, translationsRepo)
}

func TestIsModifyingLanguageDir(t *testing.T) {
	prFile1 := &github.CommitFile{
		Filename: github.String("i18n/en.json"),
	}
	prFile2 := &github.CommitFile{
		Filename: github.String("cmd/main.go"),
	}
	prFiles := []*github.CommitFile{
		prFile1,
		prFile2,
	}
	assert.True(t, isModifyingLanguageDir(prFiles, "i18n/"))
}

func TestIsNotModifyingLanguageDir(t *testing.T) {
	prFile := &github.CommitFile{
		Filename: github.String("cmd/main.go"),
	}
	prFiles := []*github.CommitFile{
		prFile,
	}
	assert.False(t, isModifyingLanguageDir(prFiles, "i18n/"))
}

func TestIsOnlyModifyingAllowedFiles(t *testing.T) {
	prFile1 := &github.CommitFile{
		Filename: github.String("i18n/en.json"),
	}
	prFile2 := &github.CommitFile{
		Filename: github.String("cmd/main.go"),
	}
	prFiles := []*github.CommitFile{
		prFile1,
		prFile2,
	}
	assert.True(t, isOnlyModifyingAllowedFiles(prFiles, "i18n/", "en.json"))
}

func TestIsNotOnlyModifyingAllowedFiles(t *testing.T) {
	prFile1 := &github.CommitFile{
		Filename: github.String("i18n/de.json"),
	}
	prFile2 := &github.CommitFile{
		Filename: github.String("cmd/main.go"),
	}
	prFile3 := &github.CommitFile{
		Filename: github.String("i18n/en.json"),
	}
	prFiles := []*github.CommitFile{
		prFile1,
		prFile2,
		prFile3,
	}
	assert.False(t, isOnlyModifyingAllowedFiles(prFiles, "i18n/", "en.json"))
}
