GO ?= $(shell command -v go 2> /dev/null)
DEP ?= $(shell command -v dep 2> /dev/null)

PACKAGES=$(shell go list ./...)

## Checks the code style, tests, builds and bundles the plugin.
.PHONY: all
all: check-style test

## Cleans workspace
.PHONY: clean
clean:
	rm -rf dist/ out/

## Checks code style.
.PHONY: check-style
check-style: golangci-lint
	@echo Checking for style guide compliance

## Run golangci-lint on codebase.
.PHONY: golangci-lint
golangci-lint:
	@if ! [ -x "$$(command -v golangci-lint)" ]; then \
		echo "golangci-lint is not installed. Please see https://github.com/golangci/golangci-lint#install for installation instructions."; \
		exit 1; \
	fi; \


	@echo Running golangci-lint
	golangci-lint run ./...

## Runs tests.
test:
	@echo Running Go tests
	$(GO) test $(PACKAGES)
	@echo test success

## Builds mattermod.
.PHONY: build
build: clean
	@echo Building
	$(GO) build -o dist/mattermod ./cmd/mattermost-mattermod

# Docker variables
DEFAULT_TAG  ?= $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)
DOCKER_IMAGE ?= mattermost/mattermod
DOCKER_TAG   ?= $(shell echo "$(DEFAULT_TAG)" | tr -d 'v')

## Build Docker image
.PHONY: docker
docker:
	docker build --pull --tag $(DOCKER_IMAGE):$(DOCKER_TAG) --file Dockerfile .

## Push Docker image
.PHONY: push
push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

## Generate mocks.
.PHONY: mocks
mocks:
	go install github.com/golang/mock/mockgen
	mockgen -package mocks -destination server/mocks/checks.go github.com/mattermost/mattermost-mattermod/server ChecksService
	mockgen -package mocks -destination server/mocks/issues.go github.com/mattermost/mattermost-mattermod/server IssuesService
	mockgen -package mocks -destination server/mocks/git.go github.com/mattermost/mattermost-mattermod/server GitService
	mockgen -package mocks -destination server/mocks/organizations.go github.com/mattermost/mattermost-mattermod/server OrganizationsService
	mockgen -package mocks -destination server/mocks/pull_requests.go github.com/mattermost/mattermost-mattermod/server PullRequestsService
	mockgen -package mocks -destination server/mocks/repositories.go github.com/mattermost/mattermost-mattermod/server RepositoriesService
	mockgen -package mocks -destination server/mocks/provider.go github.com/mattermost/mattermost-mattermod/server MetricsProvider
	mockgen -package mocks -destination store/mocks/store.go github.com/mattermost/mattermost-mattermod/store Store
	mockgen -package mocks -destination store/mocks/pull_requests.go github.com/mattermost/mattermost-mattermod/store PullRequestStore
	mockgen -package mocks -destination store/mocks/issue.go github.com/mattermost/mattermost-mattermod/store IssueStore
	mockgen -package mocks -destination store/mocks/spinmint.go github.com/mattermost/mattermost-mattermod/store SpinmintStore

# Help documentation à la https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@cat Makefile | grep -v '\.PHONY' |  grep -v '\help:' | grep -B1 -E '^[a-zA-Z_.-]+:.*' | sed -e "s/:.*//" | sed -e "s/^## //" |  grep -v '\-\-' | uniq | sed '1!G;h;$$!d' | awk 'NR%2{printf "\033[36m%-30s\033[0m",$$0;next;}1' | sort
