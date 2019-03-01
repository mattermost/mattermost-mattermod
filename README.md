# Mattermod [![CircleCI](https://circleci.com/gh/mattermost/mattermost-mattermod.svg?style=svg)](https://circleci.com/gh/mattermost/mattermost-mattermod)

Auto-generates responses to GitHub issues and pull requests on *mattermost/mattermost-server* repository using the mattermod GitHub account.

## Configuration file

To make changes to the messages simply add the appropriate `"Label"` / `"Message"` pair to [`config/config-mattermod.json`](https://github.com/mattermost/mattermost-mattermod/blob/master/config/config-mattermod.json).

Use "`USERNAME`" to have the GitHub username of the issue or pull request submitter appear.

When a change is committed, a Jenkins job will recompile and re-deploy mattermod for use on the [`mattermost/mattermost-servers`](https://github.com/mattermost/mattermost-server) repository under the mattermod GitHub account.
