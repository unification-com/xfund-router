#!/usr/bin/make -f

DEFAULT_VERSION=0.0.1

VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

# Nothing released yet - set a default
ifeq ($(strip $(VERSION)),)
VERSION=$(DEFAULT_VERSION)
endif

ldflags = -X go-ooo/version.Version=$(VERSION) \
		  -X go-ooo/version.Commit=$(COMMIT)

BUILD_FLAGS := -ldflags '$(ldflags)'

build:
	rm -f build/go-ooo && go build -mod=readonly $(BUILD_FLAGS) -o ./build/go-ooo ./

build-testapp:
	rm -f build/testapp && go build -mod=readonly $(BUILD_FLAGS) -o ./build/testapp ./testapp

install:
	go install -mod=readonly $(BUILD_FLAGS) ./

# DEPRECATED
build-release: build
	rm -rf "dist/go-ooo_v${VERSION}"
	mkdir -p "dist/go-ooo_v${VERSION}"
	cp build/go-ooo "dist/go-ooo_v${VERSION}/go-ooo"
	cd dist && tar -cpzf "go-ooo_linux_v${VERSION}.tar.gz" "go-ooo_v${VERSION}"
	cd dist && sha256sum "go-ooo_linux_v${VERSION}.tar.gz" > "checksum_v${VERSION}.txt"
	cd dist && sha256sum --check "checksum_v${VERSION}.txt"

snapshot: goreleaser
	goreleaser --snapshot --skip-publish --clean --debug

# 1. Create a new release tag on Github, e.g. v0.1.5
# 2. 'git checkout main && git pull' to get tag in local repo
# 3. run this target to generate archive & upload
release: goreleaser
	goreleaser --clean

test:
	@go test -mod=readonly ./...

test-verbose:
	@go test -mod=readonly -v ./...

lint:
	@find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" | xargs gofmt -w -s

goreleaser: $(GORELEASER)
$(GORELEASER):
	@echo "Installing goreleaser..."
	@(cd /tmp && go install github.com/goreleaser/goreleaser@latest)

.PHONY: abigen build install build-release lint test test-verbose goreleaser release snapshot
