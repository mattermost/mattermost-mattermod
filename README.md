# Mattermod [![CircleCI](https://circleci.com/gh/mattermost/mattermost-mattermod.svg?style=svg)](https://circleci.com/gh/mattermost/mattermost-mattermod)

Auto-generates responses to GitHub issues and pull requests on *mattermost/mattermost-server* repository using the mattermod GitHub account.

## Configuration file

To make changes to the messages simply add the appropriate `"Label"` / `"Message"` pair to [`config/config-mattermod.json`](https://github.com/mattermost/mattermost-mattermod/blob/master/config/config-mattermod.json).

Use "`USERNAME`" to have the GitHub username of the issue or pull request submitter appear.

When a change is committed, a Jenkins job will recompile and re-deploy mattermod for use on the [`mattermost/mattermost-servers`](https://github.com/mattermost/mattermost-server) repository under the mattermod GitHub account.

## Mattermod Local Testing

In order to test Mattermod locally a couple of steps are needed.

* 1\. Local running version of MYSQL database. You can easily spin up a database by executing ```docker run --name mattermod-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=mattermod -e MYSQL_USER=mattermod -e MYSQL_PASSWORD=mattermod -e MYSQL_DATABASE=mattermod -d mysql:5.7 > /dev/null;```
* 2\. Download the latest [`Mattermod`](https://github.com/mattermost/mattermost-mattermod) version and update the [`config/config-mattermod.json`](https://github.com/mattermost/mattermost-mattermod/blob/master/config/config-mattermod.json).
  * 2.1\. You will need to create a Github repo and a Github Access Token. *GithubAccessToken*, *Username* and *Repositories* should be updated with your details.
  * 2.2\. *DataSource* should be updated with the DSN of the MYSQL Database that was created in step 1.
  * 2.3\. *PrLabels* should be updated with your preferred PR Labels
  * 2.4\. The rest of the config file should be updated based on the testing activity. For example if you want to test the spin up of a test server, then all *SetupSpinmintTag*, *SetupSpinmintMessage*, *SetupSpinmintDoneMessage*, *SetupSpinmintFailedMessage*, *DestroyedSpinmintMessage*, *DestroyedExpirationSpinmintMessage* should be updated, as well as *JenkinsCredentials* and *AWSCredentials*.
