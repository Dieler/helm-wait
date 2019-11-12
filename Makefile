HELM_HOME ?= $(shell helm home)
PLUGIN_VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
HELM_VERSION := $(shell sed -n -e 's/version:[ "]*\([^"+]*\).*/v\1/p' plugin.yaml)

PKG:= github.com/dieler/helm-wait
LDFLAGS := -X $(PKG)/cmd.Version=$(PLUGIN_VERSION)

# Clear the "unreleased" string in BuildMetadata
LDFLAGS += -X $(PKG)/vendor/k8s.io/helm/pkg/version.BuildMetadata=
LDFLAGS += -X $(PKG)/vendor/k8s.io/helm/pkg/version.Version=$(HELM_VERSION)

.PHONY: format
format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

.PHONY: install
install: build
	mkdir -p $(HELM_HOME)/plugins/helm-wait/bin
	cp bin/wait $(HELM_HOME)/plugins/helm-wait/bin
	cp plugin.yaml $(HELM_HOME)/plugins/helm-wait/

.PHONY: build
build:
	mkdir -p bin/
	go build -i -v -o bin/wait -ldflags="$(LDFLAGS)"

.PHONY: test
test:
	go test -v ./...

PLATFORMS := windows linux darwin
os = $(word 1, $@)
binary = $(if $(findstring $(word 1, $@),windows),wait.exe,wait)
release-os = $(if $(findstring $(os),darwin),macos,$(os))

.PHONY: $(PLATFORMS)
$(PLATFORMS):
	rm -rf build/wait/*
	mkdir -p build/wait/bin
	cp README.md LICENSE plugin.yaml build/wait
	GOOS=$(os) GOARCH=amd64 go build -o build/wait/bin/$(binary) -ldflags="$(LDFLAGS)"
	mkdir -p release/
	tar -C build/ -zcvf $(CURDIR)/release/helm-wait-$(release-os).tgz wait/

.PHONY: dist
dist: windows linux darwin

.PHONY: release
release: dist
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	hub release create $(foreach file,$(wildcard release/*),--attach=$(file)) -t master v$(PLUGIN_VERSION)

BIN_DIR := $(GOPATH)/bin
GOLANGCILINT := $(BIN_DIR)/golangci-lint

$(GOLANGCILINT):
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: lint
lint: $(GOLANGCILINT)
	golangci-lint run
