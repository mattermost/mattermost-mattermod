# Project variables
NAME        := mattermost-mattermod
AUTHOR      := mattermost
URL         := https://github.com/$(AUTHOR)/$(NAME)

# Build variables
COMMIT_HASH  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_DATE   ?= $(shell date +%FT%T%z)
CUR_VERSION  ?= $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo "v0.0.0-$(COMMIT_HASH)")

# Go variables
GOLDFLAGS   :="
GOLDFLAGS   += -X github.com/$(AUTHOR)/$(NAME)/version.version=$(CUR_VERSION)
GOLDFLAGS   += -X github.com/$(AUTHOR)/$(NAME)/version.commitHash=$(COMMIT_HASH)
GOLDFLAGS   += -X github.com/$(AUTHOR)/$(NAME)/version.buildDate=$(BUILD_DATE)
GOLDFLAGS   +="

GO ?= $(shell command -v go 2> /dev/null)

PACKAGES=$(shell go list ./...)

## Checks the code style, tests and builds.
.PHONY: all
all: check-style test build

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

## Runs the mattermod server.
.PHONY: run
run: NOTILT ?=
run:
ifeq (, $(shell which tilt))
	go run ./cmd/mattermost-mattermod/main.go --config=config/config-mattermod.json
else
ifneq (, $(NOTILT))
	go run ./cmd/mattermost-mattermod/main.go --config=config/config-mattermod.json
else
	tilt up --web-mode prod
endif
endif

run-jobserver:
	go run ./cmd/jobserver --config=config/config-mattermod.json

## Runs tests.
test:
	@echo Running Go tests
	$(GO) test $(PACKAGES)
	@echo test success

## Builds mattermod binaries
.PHONY: build
build: build-mattermod build-jobserver

build-mattermod: clean
	@echo Building mattermod
	$(GO) build -ldflags $(GOLDFLAGS) -o dist/mattermod ./cmd/mattermost-mattermod

build-jobserver: clean
	@echo Building mattermod
	$(GO) build -ldflags $(GOLDFLAGS) -o dist/jobserver ./cmd/jobserver

# Docker variables
DEFAULT_TAG  ?= $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)
DOCKER_IMAGE ?= mattermost/mattermod
DOCKER_IMAGE_JOBSERVER ?= mattermost/mattermod-jobserver
DOCKER_TAG   ?= $(shell echo "$(DEFAULT_TAG)" | tr -d 'v')

## Build Docker images
.PHONY: docker
docker: docker-mattermod docker-jobserver

docker-mattermod:
	docker build --pull --tag $(DOCKER_IMAGE):$(DOCKER_TAG) --file Dockerfile .

.PHONY: docker-jobserver
docker-jobserver:
	docker build --pull --tag $(DOCKER_IMAGE_JOBSERVER):$(DOCKER_TAG) --file Dockerfile.jobserver .

## Push Docker images
.PHONY: push
push: push-mattermod push-jobserver

push-mattermod:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: push-jobserver
push-jobserver:
	docker push $(DOCKER_IMAGE_JOBSERVER):$(DOCKER_TAG)

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
	mockgen -package mocks -destination server/mocks/circleci.go github.com/mattermost/mattermost-mattermod/server CircleCIService
	mockgen -package mocks -destination store/mocks/store.go github.com/mattermost/mattermost-mattermod/store Store
	mockgen -package mocks -destination store/mocks/pull_requests.go github.com/mattermost/mattermost-mattermod/store PullRequestStore
	mockgen -package mocks -destination store/mocks/issue.go github.com/mattermost/mattermost-mattermod/store IssueStore
	mockgen -package mocks -destination store/mocks/spinmint.go github.com/mattermost/mattermost-mattermod/store SpinmintStore

#####################
## Release targets ##
#####################
PATTERN =

# if the last release was alpha, beta or rc, 'release' target has to be used with current
# cycle release. For example if latest tag is v0.8.0-rc.2 and v0.8.0 GA needs to get
# released the following should be executed: "make release VERSION=0.8.0"
## Prepare release
.PHONY: release
release: VERSION ?= $(shell git describe --tags 2>/dev/null | sed 's/^v//' | awk -F'[ .]' '{print $(PATTERN)}')
release:
	@ ./hack/release.sh "$(VERSION)" "1"

## Prepare Patch release
.PHONY: patch
patch: PATTERN = '\$$1\".\"\$$2\".\"\$$3+1'
patch: release

## Prepare Minor release
.PHONY: minor
minor: PATTERN = '\$$1\".\"\$$2+1\".0\"'
minor: release

## Prepare Major release
.PHONY: major
major: PATTERN = '\$$1+1\".0.0\"'
major: release

# Help documentation à la https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@cat Makefile | grep -v '\.PHONY' |  grep -v '\help:' | grep -B1 -E '^[a-zA-Z_.-]+:.*' | sed -e "s/:.*//" | sed -e "s/^## //" |  grep -v '\-\-' | uniq | sed '1!G;h;$$!d' | awk 'NR%2{printf "\033[36m%-30s\033[0m",$$0;next;}1' | sort
