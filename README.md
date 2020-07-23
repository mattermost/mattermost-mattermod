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

* 1\. Local running version of MYSQL database. You can easily spin up a database by executing ```docker run --name mattermod-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=mattermod -e MYSQL_USER=mattermod -e MYSQL_PASSWORD=mattermod -e MYSQL_DATABASE=mattermod -d mysql:5.7 > /dev/null;```
* 2\. Download the latest [`Mattermod`](https://github.com/mattermost/mattermost-mattermod) version and update the [`config/config-mattermod.json`](https://github.com/mattermost/mattermost-mattermod/blob/master/config/config-mattermod.json).
  * 2.1\. You will need to create a Github repo and a Github Access Token. *GithubAccessToken*, *Username* and *Repositories* should be updated with your details.
  * 2.2\. *DataSource* should be updated with the DSN of the MYSQL Database that was created in step 1.
  * 2.3\. *PrLabels* should be updated with your preferred PR Labels
  * 2.4\. The rest of the config file should be updated based on the testing activity. For example if you want to test the spin up of a test server, then all *SetupSpinmintTag*, *SetupSpinmintMessage*, *SetupSpinmintDoneMessage*, *SetupSpinmintFailedMessage*, *DestroyedSpinmintMessage*, *DestroyedExpirationSpinmintMessage* should be updated, as well as *JenkinsCredentials* and *AWSCredentials*.
