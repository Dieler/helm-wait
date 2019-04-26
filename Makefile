HELM_HOME ?= $(shell helm home)
HAS_DEP := $(shell command -v dep;)
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

.PHONY: bootstrap
bootstrap:
ifndef HAS_DEP
    go get -u github.com/golang/dep/cmd/dep
endif
	dep ensure

.PHONY: dist
dist: export COPYFILE_DISABLE=1 #teach OSX tar to not put ._* files in tar archive
dist:
	rm -rf build/wait/* release/*
	mkdir -p build/wait/bin release/
	cp README.md LICENSE plugin.yaml build/wait
	GOOS=linux GOARCH=amd64 go build -o build/wait/bin/wait -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-wait-linux.tgz wait/
	GOOS=freebsd GOARCH=amd64 go build -o build/wait/bin/wait -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-wait-freebsd.tgz wait/
	GOOS=darwin GOARCH=amd64 go build -o build/wait/bin/wait -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-wait-macos.tgz wait/
	rm build/wait/bin/wait
	GOOS=windows GOARCH=amd64 go build -o build/wait/bin/wait.exe -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-wait-windows.tgz wait/

.PHONY: release
release: dist
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	git push
	github-release dieler/helm-wait v$(PLUGIN_VERSION) master "v$(PLUGIN_VERSION)" "release/*"
