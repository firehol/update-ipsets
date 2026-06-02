GO       ?= go
BINARY   ?= update-ipsets
PREFIX   ?= /usr/local
SBINDIR  ?= $(PREFIX)/sbin
GOVULNCHECK_VERSION ?= v1.3.0
STATICCHECK_VERSION ?= v0.7.0
GOLANGCI_LINT_VERSION ?= v2.11.4
ACTIONLINT_VERSION ?= v1.7.12
GITLEAKS_VERSION ?= v8.30.1
GOVULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
STATICCHECK ?= $(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
ACTIONLINT ?= $(GO) run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
GITLEAKS ?= $(GO) run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION)
SHELLCHECK ?= shellcheck

# Version: use git tag if available, else "dev".
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

UI_STATIC_INDEX := pkg/web/static/index.html
UI_STATIC_STAMP := pkg/web/static/assets/.ui-static.stamp
UI_STATIC_INPUTS := \
	ui/package.json \
	ui/pnpm-lock.yaml \
	ui/pnpm-workspace.yaml \
	ui/index.html \
	ui/vite.config.ts \
	ui/tsconfig.json \
	ui/tsconfig.app.json \
	ui/tsconfig.node.json \
	ui/postcss.config.js \
	ui/tailwind.config.ts \
	$(shell find ui/src ui/public -type f 2>/dev/null)

.PHONY: build test test-tools test-strict fuzz-replay ui-static ui-test ui-e2e ui-budget eslint-root-config race coverage coverage-tools bench lint vulncheck staticcheck golangci-lint actionlint shellcheck gitleaks hygiene clean install cross

build: ui-static
	CGO_ENABLED=0 $(GO) build -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/update-ipsets

ui-static: $(UI_STATIC_INDEX) $(UI_STATIC_STAMP)

$(UI_STATIC_INDEX) $(UI_STATIC_STAMP) &: $(UI_STATIC_INPUTS)
	pnpm --dir ui install --frozen-lockfile
	pnpm --dir ui build
	rm -rf pkg/web/static/assets
	mkdir -p pkg/web/static/assets
	cp -R ui/dist/assets/. pkg/web/static/assets/
	cp ui/dist/index.html $(UI_STATIC_INDEX)
	touch $(UI_STATIC_STAMP)

test: ui-static
	$(GO) test ./...

test-tools:
	cd tools/dronebl2ipsets && $(GO) test ./...

test-strict: ui-static
	$(GO) test -shuffle=on -count=3 ./pkg/scheduler ./pkg/engine ./pkg/web

fuzz-replay:
	$(GO) test -run=Fuzz ./pkg/iprange ./pkg/config ./pkg/processor

ui-test:
	pnpm --dir ui test
	pnpm --dir ui test:bundle-budget

ui-e2e:
	pnpm --dir ui test:e2e

ui-budget:
	pnpm --dir ui build:budget

eslint-root-config:
	pnpm --dir ui test:eslint-root-config

race: ui-static
	$(GO) test -race ./...
	cd tools/dronebl2ipsets && $(GO) test -race ./...

coverage: ui-static
	$(GO) test -coverprofile=coverage.out -covermode=atomic ./...

coverage-tools:
	cd tools/dronebl2ipsets && $(GO) test -coverprofile=coverage.out -covermode=atomic ./...

bench:
	$(GO) test -bench=. -benchmem ./...

lint:
	$(GO) vet ./...

vulncheck:
	$(GOVULNCHECK) ./...
	cd tools/dronebl2ipsets && $(GOVULNCHECK) ./...

staticcheck:
	$(STATICCHECK) ./...
	cd tools/dronebl2ipsets && $(STATICCHECK) ./...

golangci-lint:
	$(GOLANGCI_LINT) run ./...
	cd tools/dronebl2ipsets && $(GOLANGCI_LINT) run ./...

actionlint:
	$(ACTIONLINT) .github/workflows/*.yml

shellcheck:
	$(SHELLCHECK) $(shell git ls-files '*.sh')

gitleaks:
	$(GITLEAKS) detect --no-banner --redact=100 --source . --exit-code 2

hygiene: actionlint shellcheck gitleaks

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 coverage.out bench.out tools/dronebl2ipsets/coverage.out

install: build
	install -d $(DESTDIR)$(SBINDIR)
	install -m 0755 $(BINARY) $(DESTDIR)$(SBINDIR)/$(BINARY)

# Cross-compile static binaries for linux/amd64 and linux/arm64.
cross: ui-static
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -ldflags '$(LDFLAGS)' -o $(BINARY)-linux-amd64 ./cmd/update-ipsets
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -ldflags '$(LDFLAGS)' -o $(BINARY)-linux-arm64 ./cmd/update-ipsets
