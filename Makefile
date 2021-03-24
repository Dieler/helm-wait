PLUGIN_VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
HELM_VERSION := $(shell sed -n -e 's/version:[ "]*\([^"+]*\).*/v\1/p' plugin.yaml)
HELM_PLUGINS := $(shell bash -c 'eval $$(helm env); echo $$HELM_PLUGINS')


PKG:= github.com/dieler/helm-wait
LDFLAGS := -X $(PKG)/cmd.Version=$(PLUGIN_VERSION)

.PHONY: deps
deps:
	go get github.com/spf13/cobra@v1.1.3
	go get github.com/spf13/pflag@v1.0.5
	go get gopkg.in/yaml.v2@v2.3.0
	go get github.com/mgutz/ansi
	go get helm.sh/helm/v3@v3.5.1
	go get k8s.io/client-go@v0.19.9

.PHONY: format
format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

.PHONY: install
install: build
	mkdir -p $(HELM_PLUGINS)/helm-wait/bin
	cp bin/wait $(HELM_PLUGINS)/helm-wait/bin
	cp plugin.yaml $(HELM_PLUGINS)/helm-wait/

.PHONY: build
build:
	mkdir -p bin/
	go build -v -o bin/wait -ldflags="$(LDFLAGS)"

.PHONY: test
test:
	go test -v ./...

.PHONY: vet
vet:
	go vet -v ./...

GOLANGLINT := golint
$(GOLANGLINT):
	go get golang.org/x/lint/golint

.PHONY: lint
lint: $(GOLANGLINT)
	golint ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: test-plugin-installation
test-plugin-installation:
	docker build -f testdata/Dockerfile.install .

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
	tar -C build/wait -zcvf $(CURDIR)/release/helm-wait-$(release-os).tgz .

.PHONY: dist
dist: windows linux darwin

.PHONY: release
release: dist
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	hub release create $(foreach file,$(wildcard release/*),--attach=$(file)) -t master v$(PLUGIN_VERSION)
