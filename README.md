# Mattermod

Auto-generates responses to GitHub issues and pull requests on mattermost/platform repository using the mattermod GitHub account. 

## Configuration file 

To make changes to the messages simply add the appropreate `"Label"` / `"Message"` pair to [`config/config-mattermod.json`](https://github.com/mattermost/mattermod/blob/master/config/config-mattermod.json).

Use "`USERNAME`" to have the GitHub username of the issue or pull request submitter appear.

When a change is committed, a Jenkins job will recompile and re-deploy mattermod for use on the [`mattermost/platform`](https://github.com/mattermost/platform) repository under the mattermod GitHub account. 
