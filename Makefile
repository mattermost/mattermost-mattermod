GO ?= $(shell command -v go 2> /dev/null)
DEP ?= $(shell command -v dep 2> /dev/null)

PACKAGES=$(shell go list ./...)

## Checks the code style, tests, builds and bundles the plugin.
.PHONY: all
all: check-style test

## Runs govet and gofmt against all packages.
.PHONY: check-style
check-style: gofmt goimports govet
	@echo Checking for style guide compliance

build:
	@echo Building

	rm -rf dist/
	mkdir -p dist/mattermod
	$(GO) build
	mv mattermost-mattermod dist/mattermod/
	cp config/config-mattermod.default.json dist/mattermod/config-mattermod.json


package: gofmt goimports govet build
	tar -C dist -czf dist/mattermod.tar.gz mattermod

## Runs gofmt against all packages.
.PHONY: gofmt
gofmt:
	@echo Running GOFMT

	@for package in $(PACKAGES) ; do \
		echo "Checking "$$package; \
		files=$$(go list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}' $$package); \
		if [ "$$files" ]; then \
			gofmt_output=$$(gofmt -d -s $$files 2>&1); \
			if [ "$$gofmt_output" ]; then \
				echo "$$gofmt_output"; \
				echo "gofmt failure"; \
				exit 1; \
			fi; \
		fi; \
	done
	@echo "gofmt success"; \

## Runs goimports against go files.
.PHONY: goimports
goimports:
	@echo Running goimports
	env GO111MODULE=off $(GO) get golang.org/x/tools/cmd/goimports
	! goimports -d `find . -type f -name '*.go' -not -path "./vendor/*"` | grep .

## Runs govet against all packages.
.PHONY: govet
govet:
	@echo Running govet
	env GO111MODULE=off $(GO) get golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow
	$(GO) vet $(PACKAGES) || exit 1
	$(GO) vet -vettool=$(GOPATH)/bin/shadow $(PACKAGES) || exit 1
	@echo Govet success

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
