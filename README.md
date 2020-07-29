# Mattermod [![CircleCI](https://circleci.com/gh/mattermost/mattermost-mattermod.svg?style=svg)](https://circleci.com/gh/mattermost/mattermost-mattermod)

Auto-generates responses to GitHub issues and pull requests on *mattermost/mattermost-server* repository using the mattermod GitHub account.

## Developing

### Environment Setup

Essentials:

- Install [Go](https://golang.org/doc/install)

Optionals:

- [Tilt](https://tilt.dev/) v0.13+ (to deploy on a local dev K8s cluster)
- [kind](https://kind.sigs.k8s.io/) v0.8+ (to spin up a local dev K8s cluster)
  - for better performance use [kind-with-registry.sh](https://github.com/tilt-dev/kind-local#how-to-try-it)
- [kustomize](https://github.com/kubernetes-sigs/kustomize) v3.6+

### Running

This project uses `tilt` to deploy to local Kubernetes cluster. In order to do this you need a local Kuberetes cluster (`kind` is recommended).

```bash
KIND_CLUSTER_NAME=mattermod /path/to/kind-with-registry.sh

# Or directly with kind, which is gonna be less performant
# kind create cluster --name mattermod
```

Mattermod deployment to any cluster and any environment (dev, prod, etc) depends on existense of `deploy/base/config/config.json` file, this file is `.gitignore`d and you can safeley choose to copy sample config there for local development and testing:

```shell
cp config.json deploy/base/config/
```

**Note:** for local development make sure `DataSource` in `config.json` is set to the following:

```txt
root:password@tcp(mysql.default.svc.cluster.local:3306)/mattermod?parseTime=true
```

Point `KUBECONFIG` to the newly created cluster, and start `tilt` and open [http://localhost:8080/](http://localhost:8080/):

```shell
make run
```

**Note:** If you don't want to use Tilt nor deploy to local cluster you can ignore it and simply start the binary server:

```bash
NOTILT=1 make run
```

### Testing

Running all tests:

```shell
make test
```

Generate github mocks:

```shell
make mocks
```

## Configuration file

To make changes to the messages simply add the appropriate `"Label"` / `"Message"` pair to [`config/config-mattermod.json`](https://github.com/mattermost/mattermost-mattermod/blob/master/config/config-mattermod.json).

Use "`USERNAME`" to have the GitHub username of the issue or pull request submitter appear.

When a change is committed, a Jenkins job will recompile and re-deploy mattermod for use on the [`mattermost/mattermost-servers`](https://github.com/mattermost/mattermost-server) repository under the mattermod GitHub account.

## Mattermod Local Testing

In order to test Mattermod locally a couple of steps are needed.

1. Since Mattermod needs to consume webhooks from GitHub, we need to expose our webserver to the public internet. It is recommended to use [ngrok](https://ngrok.com/) for this, but you are free to use anything else.

    For Mattermost staff, please visit https://dashboard.ngrok.com/endpoints/domains and reserve a domain under your name, under the correct region. As a convention, please use `<githubusername>.<region>.ngrok.io`.

    Download ngrok locally and setup your authtoken from this page: https://dashboard.ngrok.com/get-started/setup.

    Fire up ngrok with `./ngrok http -subdomain=<githubusername> -region=<region> 8086`.

2. Go to https://github.com/mattermosttest/mattermost-server/settings/hooks and add your webhook (If you don't have access to that URL, ping in the [Mattermod](https://community-daily.mattermost.com/private-core/channels/mattermod-logs) channel). The payload URL will point to your ngrok URL. For now the path should be at "/pr_event", but it is going to be split into multiple paths later.

    Create a webhook secret. This can be any random string. Note it down.

3. Go to https://github.com/settings/tokens and generate an access token. Note it down.

4. We also need to have a MySQL DB instance running. If you are using docker, use the following command:
```
docker run --name mattermod-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=mattermod -e MYSQL_USER=mattermod -e MYSQL_PASSWORD=mattermod -e MYSQL_DATABASE=mattermod -d mysql:5.7 > /dev/null;
```

5. Now we need to populate `config-mattermod.json` with the right values.
- Set `GithubAccessToken` and `GithubAccessTokenCherryPick` with your GitHub access token.
- Set `GithubWebhookSecret` with the webhook secret.
- Set `Org`, `GitHubUsername` correctly. For Mattermost staff, org should be `mattermosttest`.
- `DataSource` should be updated with the DSN of the MYSQL Database
- The rest of the config file should be updated based on the testing activity. For example if you want to test the spin up of a test server, then all `SetupSpinmintTag`, `SetupSpinmintMessage`, `SetupSpinmintDoneMessage`, `SetupSpinmintFailedMessage`, `DestroyedSpinmintMessage`, `DestroyedExpirationSpinmintMessage` should be updated, as well as `JenkinsCredentials` and `AWSCredentials`.

    For any other relevant config which is missing, please see https://github.com/mattermost/platform-private/blob/master/mattermod/config.json.

6. For cherry-picking to work, [hub](https://github.com/github/hub) needs to be installed in the system, and the script from `hacks/cherry-pick.sh` should be placed at `/app/scripts/cherry-pick.sh`.

7. Start up Mattermod server.
