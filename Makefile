# ── NullGate WAF — Makefile ──────────────────────────────────────────────────
# Common developer and operator tasks. Run `make help` for the full list.

BINARY      := nullgate
CMD_PKG     := ./cmd/nullgate
VERSION     := 2.0.0
BUILD_DIR   := bin
LDFLAGS     := -s -w -X main.Version=$(VERSION)
GOFILES     := $(shell go list ./... 2>/dev/null)

# Cross-compilation matrix for release artifacts.
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.DEFAULT_GOAL := help

## help: Show this help.
.PHONY: help
help:
	@echo "NullGate WAF v$(VERSION) — make targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: Build the WAF binary for the host platform.
.PHONY: build
build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) $(CMD_PKG)

## run: Build and run against configs/config.yaml.
.PHONY: run
run: build
	./$(BINARY) --config configs/config.yaml

## dashboard: Build and run with the live control panel on 127.0.0.1:9091.
.PHONY: dashboard
dashboard:
	bash scripts/dashboard.sh

## test: Run the full test suite.
.PHONY: test
test:
	go test ./... -count=1

## test-v: Run tests verbosely.
.PHONY: test-v
test-v:
	go test ./... -count=1 -v

## cover: Run tests with a coverage report.
.PHONY: cover
cover:
	go test ./internal/... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1
	@echo "HTML report: go tool cover -html=coverage.out"

## bench: Run benchmarks.
.PHONY: bench
bench:
	go test ./internal/waf/ -bench=. -benchmem -run=^$$

## selftest: Build and run the detection self-test (competition gate).
.PHONY: selftest
selftest: build
	./$(BINARY) --config configs/config.yaml --selftest

## vet: Run go vet static analysis.
.PHONY: vet
vet:
	go vet ./...

## fmt: Format all Go source.
.PHONY: fmt
fmt:
	gofmt -s -w .

## lint: fmt + vet + test (the pre-commit gate).
.PHONY: lint
lint: fmt vet test

## gen-secret: Print a fresh random HMAC secret for pow.hmac_secret.
.PHONY: gen-secret
gen-secret: build
	@./$(BINARY) --gen-secret

## show-config: Print the effective configuration after defaults/clamping.
.PHONY: show-config
show-config: build
	./$(BINARY) --config configs/config.yaml --show-config

## release: Cross-compile stripped binaries for every target platform.
.PHONY: release
release:
	@mkdir -p $(BUILD_DIR)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "  building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath \
			-ldflags="$(LDFLAGS)" \
			-o $(BUILD_DIR)/$(BINARY)-$$os-$$arch$$ext $(CMD_PKG); \
	done
	@echo "release artifacts in $(BUILD_DIR)/"

## docker: Build the Docker image.
.PHONY: docker
docker:
	docker build --build-arg VERSION=$(VERSION) -t nullgate:$(VERSION) .

## up: Bring up the full demo stack (WAF + vulnerable backend).
.PHONY: up
up:
	docker compose up --build

## down: Tear down the demo stack.
.PHONY: down
down:
	docker compose down -v

## clean: Remove build artifacts.
.PHONY: clean
clean:
	rm -f $(BINARY) $(BINARY).exe coverage.out
	rm -rf $(BUILD_DIR)
