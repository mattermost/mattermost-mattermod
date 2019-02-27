# Mattermod [![CircleCI](https://circleci.com/gh/mattermost/mattermost-mattermod.svg?style=svg)](https://circleci.com/gh/mattermost/mattermost-mattermod)

Auto-generates responses to GitHub issues and pull requests on *mattermost/mattermost-server* repository using the mattermod GitHub account.

## Configuration file

To make changes to the messages simply add the appropriate `"Label"` / `"Message"` pair to [`config/config-mattermod.json`](https://github.com/mattermost/mattermost-mattermod/blob/master/config/config-mattermod.json).

Use "`USERNAME`" to have the GitHub username of the issue or pull request submitter appear.

When a change is committed, a Jenkins job will recompile and re-deploy mattermod for use on the [`mattermost/mattermost-servers`](https://github.com/mattermost/mattermost-server) repository under the mattermod GitHub account.

## Mattermod Local Testing

In order to test Mattermod locally a couple of steps are needed.

* 1\. Local running version of **Mattermost Server**. Please see [`mattermost-server-setup`](https://developers.mattermost.com/contribute/server/developer-setup/) for more information.
* 2\. Once Mattermost Server is running successfully copy **TEST_DATABASE_MYSQL_DSN** from  *mattermost-server/build/local-test-env.sh*, as you will need this for Mattermod MYSQL configuration.
* 3\. In case you want to test the webhook functionality, you will need to run locally the **Mattermost Web App**; please see [`mattermost-webapp-setup`](https://developers.mattermost.com/contribute/webapp/) for more information.
* 4\. Once the web app is up and running, check [`mattermost-webhooks`](https://docs.mattermost.com/developer/webhooks-incoming.html) page to enable webhooks. Create a webhook and copy the details, as you will need this for Mattermod webhook configuration.
* 5\. Download the latest [`Mattermod`](https://github.com/mattermost/mattermost-mattermod) version and update the [`config/config-mattermod.json`](https://github.com/mattermost/mattermost-mattermod/blob/master/config/config-mattermod.json).
  * 5.1\. You will need to create a private Github repo and a Github Access Token. *GithubAccessToken*, *Username* and *Repositories* should be updated with your details.
  * 5.2\. *DataSource* should be updated with the information from step 2.
  * 5.3\. *PrLabels* should be updated with your preferred PR Labels
  * 5.3\. *MattermostWebhookURL* should be updated with the details from step 4.
  * 5.4\. The rest of the config file should be updated based on the testing activity. For example if you want to test the spin up of a test server, then all *SetupSpinmintTag*, *SetupSpinmintMessage*, *SetupSpinmintDoneMessage*, *SetupSpinmintFailedMessage*, *DestroyedSpinmintMessage*, *DestroyedExpirationSpinmintMessage* should be updated, as well as *JenkinsCredentials* and *AWSCredentials*.
