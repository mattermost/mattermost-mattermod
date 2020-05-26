GO ?= $(shell command -v go 2> /dev/null)
DEP ?= $(shell command -v dep 2> /dev/null)

PACKAGES=$(shell go list ./...)

## Checks the code style, tests, builds and bundles the plugin.
.PHONY: all
all: check-style test

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

build:
	@echo Building

	rm -rf dist/
	mkdir -p dist/mattermod
	$(GO) build
	mv mattermost-mattermod dist/mattermod/
	cp config/config-mattermod.default.json dist/mattermod/config-mattermod.json


package: gofmt goimports govet build
	tar -C dist -czf dist/mattermod.tar.gz mattermod

## Runs tests. For local usage, run `make test CONFIG_TEST="-config=config-mattermod.test-local.json"`
test:
	@echo Running Go tests
	$(GO) test $(PACKAGES) $(CONFIG_TEST)
	@echo test success


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

# Help documentation à la https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@cat Makefile | grep -v '\.PHONY' |  grep -v '\help:' | grep -B1 -E '^[a-zA-Z_.-]+:.*' | sed -e "s/:.*//" | sed -e "s/^## //" |  grep -v '\-\-' | sed '1!G;h;$$!d' | awk 'NR%2{printf "\033[36m%-30s\033[0m",$$0;next;}1' | sort
