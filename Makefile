GO ?= go
NPM ?= npm
VERSION ?= $(shell tr -d '[:space:]' < VERSION)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || cat RELEASE_COMMIT 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -s -w -X qmi-web/internal/version.Version=$(VERSION) -X qmi-web/internal/version.Commit=$(COMMIT) -X qmi-web/internal/version.BuildTime=$(BUILD_TIME)
GO_PACKAGES = ./cmd/... ./internal/...

.PHONY: dev build test frontend backend image docker offline-build package-offline verify ownership-test clean security-check

dev:
	QMI_WEB_BACKEND=mock $(GO) run -mod=vendor ./cmd/server

frontend:
	rm -rf internal/web/dist
	mkdir -p internal/web/dist
	cd web && $(NPM) ci && $(NPM) run typecheck && $(NPM) test && $(NPM) run build
	test -f internal/web/dist/index.html

backend:
	$(GO) build -mod=vendor -buildvcs=false -trimpath -ldflags="$(LDFLAGS)" -o build/qmi-web ./cmd/server

build: frontend backend

test:
	$(GO) test -mod=vendor $(GO_PACKAGES)
	$(GO) vet -mod=vendor $(GO_PACKAGES)
	cd web && $(NPM) ci && $(NPM) test

image: build
	docker build --pull=false --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_TIME=$(BUILD_TIME) -t local/qmi-web:$(VERSION) .

docker: image

offline-build:
	./scripts/offline-build.sh

package-offline:
	./scripts/package-offline.sh

verify:
	./scripts/verify-security.sh
	./scripts/host/test-ownership.sh
	$(GO) test -mod=vendor $(GO_PACKAGES)
	$(GO) vet -mod=vendor $(GO_PACKAGES)

ownership-test:
	./scripts/host/test-ownership.sh

security-check:
	./scripts/verify-security.sh

clean:
	rm -rf build web/node_modules web/dist internal/web/dist/assets
	mkdir -p internal/web/dist/assets
	cp internal/web/placeholder/index.html internal/web/dist/index.html
	cp internal/web/placeholder/assets/app-placeholder.js internal/web/dist/assets/app-placeholder.js
