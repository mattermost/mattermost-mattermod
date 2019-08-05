package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	jenkins "github.com/cpanato/golang-jenkins"
	"github.com/mattermost/mattermost-mattermod/model"
	"github.com/mattermost/mattermost-server/mlog"
	"github.com/pkg/errors"
)

func (s *Server) buildJenkinsClient(pr *model.PullRequest) (*Repository, *jenkins.Jenkins, error) {
	repo, ok := s.GetRepository(pr.RepoOwner, pr.RepoName)
	if !ok || repo.JenkinsServer == "" {
		return repo, nil, errors.New("jenkins server is not configured")
	}
	credentials, ok := s.Config.JenkinsCredentials[repo.JenkinsServer]
	if !ok {
		return repo, nil, errors.New("jenkins server credentials are not configured")
	}

	client := jenkins.NewJenkins(&jenkins.Auth{
		Username: credentials.Username,
		ApiToken: credentials.ApiToken,
	}, credentials.URL)

	return repo, client, nil
}

func (s *Server) waitForBuild(ctx context.Context, client *jenkins.Jenkins, pr *model.PullRequest) (*model.PullRequest, error) {
	for {
		select {
		case <-ctx.Done():
			return pr, errors.New("timed out waiting for build to finish")
		case <-time.After(30 * time.Second):
			result := <-s.Store.PullRequest().Get(pr.RepoOwner, pr.RepoName, pr.Number)
			if result.Err != nil {
				return pr, errors.Wrap(result.Err, "unable to get updated PR from Mattermod database")
			}

			// Update the PR in case the build link has changed because of a new commit
			pr = result.Data.(*model.PullRequest)
			var err error
			pr, err = s.GetUpdateChecks(pr.RepoOwner, pr.RepoName, pr.Number)
			if err != nil {
				return pr, errors.Wrap(err, "unable to get updated PR from GitHub")
			}
			mlog.Info("Current PR Status", mlog.String("repo_name", pr.RepoName), mlog.String("build_status", pr.BuildStatus), mlog.String("build_conclusion", pr.BuildConclusion))

			if pr.RepoName == "mattermost-webapp" {
				switch pr.BuildStatus {
				case "in_progress":
					mlog.Info("Build in CircleCI is still in progress")
				case "completed":
					if pr.BuildConclusion == "success" {
						mlog.Info("Build in CircleCI succeed")
						return pr, nil
					}
					return pr, errors.New("build failed")
				default:
					return pr, fmt.Errorf("unknown build status %s", pr.BuildStatus)
				}
			} else {
				if pr.BuildLink == "" {
					mlog.Info("No build link found; skipping...")
				} else {
					mlog.Info("BuildLink for PR", mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName), mlog.String("buildlink", pr.BuildLink))
					// Doing this because the lib we are using does not support folders :(
					var jobNumber int64
					var jobName string

					parts := strings.Split(pr.BuildLink, "/")
					// Doing this because the lib we are using does not support folders :(
					switch pr.RepoName {
					case "mattermost-server":
						jobNumber, _ = strconv.ParseInt(parts[len(parts)-3], 10, 32)
						jobName = parts[len(parts)-6]     //mattermost-server
						subJobName := parts[len(parts)-4] //PR-XXXX
						jobName = "mp/job/" + jobName + "/job/" + subJobName
					case "mattermost-mobile":
						jobNumber, _ = strconv.ParseInt(parts[len(parts)-2], 10, 32)
						jobName = parts[len(parts)-3] //mattermost-mobile
						jobName = "mm/job/" + jobName
					default:
						return pr, fmt.Errorf("unsupported repository %s", pr.RepoName)
					}

					job, err := client.GetJob(jobName)
					if err != nil {
						return pr, errors.Wrapf(err, "failed to get Jenkins job %s", jobName)
					}

					// Doing this because the lib we are using does not support folders :(
					// This time is in the Jenkins job Name because it returns just the name
					job.Name = jobName

					build, err := client.GetBuild(job, int(jobNumber))
					if err != nil {
						return pr, errors.Wrapf(err, "failed to get Jenkins build %d", build.Number)
					}

					if !build.Building && build.Result == "SUCCESS" {
						mlog.Info("build for PR succeeded!", mlog.Int("build_number", build.Number), mlog.Int("pr", pr.Number), mlog.String("repo_owner", pr.RepoOwner), mlog.String("repo_name", pr.RepoName))
						return pr, nil
					} else if build.Result == "FAILURE" || build.Result == "ABORTED" {
						return pr, fmt.Errorf("build %d failed with status %q", build.Number, build.Result)
					} else {
						mlog.Info("Build is running", mlog.Int("build", build.Number), mlog.Bool("building", build.Building))
					}
				}
			}

			mlog.Info("Build is still in progress; sleeping...")
		}
	}
}

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
