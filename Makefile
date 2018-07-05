GOFLAGS ?= $(GOFLAGS:)
GO ?= $(GO:vgo)

# GOOS/GOARCH of the build host, used to determine whether we're cross-compiling or not
BUILDER_GOOS_GOARCH="$(shell $(GO) env GOOS)_$(shell $(GO) env GOARCH)"
DIST_PATH=.

all: install

.PHONY: vgo
vgo:
	go get -u golang.org/x/vgo

.PHONY: build-linux
build-linux: vgo
	@echo Build Linux amd64
	env GOOS=linux GOARCH=amd64 $(GO) install -i $(GOFLAGS) $(GO_LINKER_FLAGS) ./...

.PHONY: build-osx
build-osx: vgo
	@echo Build OSX amd64
	env GOOS=darwin GOARCH=amd64 $(GO) install -i $(GOFLAGS) $(GO_LINKER_FLAGS) ./...

.PHONY: build-window
build-windows: vgo
	@echo Build Windows amd64
	env GOOS=windows GOARCH=amd64 $(GO) install -i $(GOFLAGS) $(GO_LINKER_FLAGS) ./...

build: build-linux build-windows build-osx

# Build and install for the current platform
.PHONY: install
install:
ifeq ($(BUILDER_GOOS_GOARCH),"darwin_amd64")
	@$(MAKE) build-osx
endif
ifeq ($(BUILDER_GOOS_GOARCH),"windows_amd64")
	@$(MAKE) build-windows
endif
ifeq ($(BUILDER_GOOS_GOARCH),"linux_amd64")
	@$(MAKE) build-linux
endif

package: build-linux
	@# ----- PLATFORM SPECIFIC -----

	@# Linux, the only supported package version for now. Build manually for other targets.
ifeq ($(BUILDER_GOOS_GOARCH),"linux_amd64")
	cp $(GOPATH)/bin/mattermost-mattermod $(DIST_PATH)/ # from native bin dir, not cross-compiled
else
	cp $(GOPATH)/bin/linux_amd64/mattermost-mattermod $(DIST_PATH)/ # from cross-compiled bin dir
endif

.PHONY: clean
clean:
	rm -f mattermost-mattermod
