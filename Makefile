GO ?= $(shell command -v go 2> /dev/null)
DEP ?= $(shell command -v dep 2> /dev/null)

PACKAGES=$(shell go list ./...)

## Checks the code style, tests, builds and bundles the plugin.
.PHONY: all
all: check-style test

## Runs govet and gofmt against all packages.
.PHONY: check-style
check-style: gofmt govet
	@echo Checking for style guide compliance

build:
	@echo Building

	rm -rf dist/
	mkdir -p dist/mattermod
	$(GO) build -mod=vendor
	mv mattermost-mattermod dist/mattermod/
	cp config/config-mattermost.default.json dist/mattermod/config-mattermod.json


package: gofmt govet build
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

## Runs govet against all packages.
.PHONY: govet
govet:
	@echo Running govet
	env GO111MODULE=off $(GO) get golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow
	$(GO) vet -mod=vendor $(PACKAGES) || exit 1
	$(GO) vet -mod=vendor -vettool=$(GOPATH)/bin/shadow $(PACKAGES) || exit 1
	@echo Govet success

## Runs tests. For local usage, run `make test CONFIG_TEST="-config=config-mattermod.test-local.json"`
test:
	@echo Running Go tests
	make test CONFIG_TEST="-config=config-mattermod.local.json"
	$(GO) test -mod=vendor $(PACKAGES) $(CONFIG_TEST)
	@echo test success

# Help documentation à la https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@cat Makefile | grep -v '\.PHONY' |  grep -v '\help:' | grep -B1 -E '^[a-zA-Z_.-]+:.*' | sed -e "s/:.*//" | sed -e "s/^## //" |  grep -v '\-\-' | sed '1!G;h;$$!d' | awk 'NR%2{printf "\033[36m%-30s\033[0m",$$0;next;}1' | sort
