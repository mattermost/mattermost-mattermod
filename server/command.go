package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/pkg/errors"
)

const (
	CherryPick        = "cherry-pick"
	GoImportsLocal    = "goimports-local"
	tooManyCommandMsg = "There are too many command requests. Please do this manually or try again later."
)

type commandRequest struct {
	pr        *model.PullRequest
	command   string
	cmdArgs   []string
	version   string
	milestone *int
}

func (s *Server) handleCommandRequest(ctx context.Context, commenter, command, body string, pr *model.PullRequest) error {
	var msg string
	defer func() {
		if msg != "" {
			if err := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	}()

	// trigger `goimports -local` for any user
	if command != GoImportsLocal && !s.IsOrgMember(commenter) {
		msg = msgCommenterPermission
		return nil
	}
	commandIndex := getCommandIndex(body, command)
	args := strings.Split(body[commandIndex:], " ")
	mlog.Info("Command & Args", mlog.String("Command, ", command), mlog.String("Args", body))

	if pr.GetMerged() {
		return nil
	}

	select {
	case <-s.commandStopChan:
		return errors.New("server is closing")
	default:
	}

	version := "0"
	if command == CherryPick {
		if len(args) < 2 {
			return nil
		}
		version = strings.TrimSpace(args[1])
	}

	select {
	case s.commandRequests <- &commandRequest{
		pr:      pr,
		version: version,
		command: command,
		cmdArgs: args,
	}:
		switch command {
		case CherryPick:
			msg = cherryPickScheduledMsg
		case GoImportsLocal:
			msg = goimportsLocalScheduledMsg
		}
	default:
		msg = tooManyCommandMsg
		return errors.New("too many requests")
	}

	return nil
}

func (s *Server) listenCommandRequests() {
	defer func() {
		close(s.commandStoppedChan)
	}()

	for job := range s.commandRequests {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*2*time.Second)
			defer cancel()
			pr := job.pr
			command := job.command
			switch command {
			case CherryPick:
				cmdOut, err := s.doCherryPick(ctx, strings.TrimSpace(job.version), job.milestone, pr)
				if err != nil {
					msg := fmt.Sprintf("Error trying doing the automated Cherry picking. Please do this manually\n\n```\n%s\n```\n", cmdOut)
					if cErr := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); cErr != nil {
						mlog.Warn("Error while commenting", mlog.Err(cErr))
					}
					mlog.Error("Error while cherry picking", mlog.Err(err))
				}
			case GoImportsLocal:
				cmdOut, err := s.doGoImportsLocal(ctx, pr)
				if err != nil {
					msg := fmt.Sprintf("Error trying doing the automated goimports -local run. Please do this manually\n\n```\n%s\n```\n", cmdOut)
					if cErr := s.sendGitHubComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, msg); cErr != nil {
						mlog.Warn("Error while commenting", mlog.Err(cErr))
					}
					mlog.Error("Error while running goimports -local", mlog.Err(err))
				}
			}
		}()
	}
}

func (s *Server) finishCommandRequests() {
	close(s.commandStopChan)
	close(s.commandRequests)
	select {
	case <-time.After(5 * time.Second):
		ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout*time.Second)
		defer cancel()
		// While consuming requests here, listenCommandRequests routine will continue
		// to listen as well. We're just trying to cancel requests as much as we can.
		msg := "Commands execution is canceled due to server shutdown."
		for job := range s.commandRequests {
			if err := s.sendGitHubComment(ctx, job.pr.RepoOwner, job.pr.RepoName, job.pr.Number, msg); err != nil {
				mlog.Warn("Error while commenting", mlog.Err(err))
			}
		}
	case <-s.commandStoppedChan:
	}
}

func getCommandIndex(body, command string) int {
	return strings.Index(body, "/"+command)
}
