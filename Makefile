# Petti — Docker-like container runtime (educational)
MODULE      := github.com/r4r00t-sh/petti
BINARY      := bin/petti
CMD         := ./cmd/petti

VERSION     ?= 0.1.0
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE  ?= $(shell date -u +%Y-%m-%d)
GO          ?= go

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build test lint integration clean vet race coverage release-local

build:
	@mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

# Cross-compile static binaries into dist/ (same layout as the release workflow).
release-local:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" \
		-o dist/petti-linux-amd64 $(CMD)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" \
		-o dist/petti-linux-arm64 $(CMD)
	tar -C dist -czf dist/petti-linux-amd64.tar.gz petti-linux-amd64
	tar -C dist -czf dist/petti-linux-arm64.tar.gz petti-linux-arm64
	(cd dist && sha256sum petti-linux-amd64.tar.gz petti-linux-arm64.tar.gz > SHA256SUMS)
	@ls -la dist
	@file dist/petti-linux-amd64 dist/petti-linux-arm64

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

race:
	$(GO) test -race ./...

coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found; install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; \
		exit 1; \
	}
	golangci-lint run ./...

# Integration tests require Linux + root (+ busybox for the fixture rootfs).
# -p 1 avoids parallel packages racing on cgroups/iptables/veth and exhausting PIDs.
# GOTMPDIR/TMPDIR on disk keeps rootful t.TempDir fixtures off small /tmp tmpfs.
# PETTI_INIT must be the real CLI: go test binaries do not handle __init re-exec.
integration: build
	@mkdir -p .tmp
	GOTMPDIR=$(CURDIR)/.tmp TMPDIR=$(CURDIR)/.tmp PETTI_INIT=$(CURDIR)/$(BINARY) \
		$(GO) test -tags=integration -count=1 -p 1 ./...

clean:
	rm -rf bin/ dist/ coverage.out coverage.html
