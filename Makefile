## ObsidianStack Makefile
## Usage: make <target>
## Go 1.24 is required — detected automatically via GOBIN below.

GOBIN   := $(shell command -v /opt/homebrew/opt/go@1.24/bin/go 2>/dev/null || command -v go)
BIN     := bin
AGENT   := $(BIN)/obsidianstack-agent
SERVER  := $(BIN)/obsidianstack-server
MODULE  := github.com/obsidianstack/obsidianstack

# Config files used for local development (git-ignored, contain real endpoints/secrets)
DEV_AGENT_CFG  := config/agent.yaml
DEV_SERVER_CFG := config/server.yaml

.PHONY: all build build-agent build-server proto lint test tidy clean \
        run-server run-agent run-ui run-portfwd stop \
        docker-agent docker-server up down help

all: build ## Default: build both binaries

## ── Build ───────────────────────────────────────────────────────────────────

build: build-agent build-server ## Compile agent + server into bin/

build-agent: $(BIN)
	$(GOBIN) build -o $(AGENT) ./agent/cmd/agent

build-server: $(BIN)
	$(GOBIN) build -o $(SERVER) ./server/cmd/server

$(BIN):
	mkdir -p $(BIN)

## ── Local dev ───────────────────────────────────────────────────────────────

## run-server: start the server (config/server.yaml)
run-server: build-server
	$(SERVER) -config $(DEV_SERVER_CFG)

## run-agent: start the agent (config/agent.yaml)
## Set PROM_PASSWORD etc. in your shell before running:
##   export PROM_PASSWORD=yourpassword && make run-agent
run-agent: build-agent
	$(AGENT) -config $(DEV_AGENT_CFG)

## run-ui: start the Vite dev server on localhost:3000
run-ui:
	cd ui && npm run dev

## run-portfwd: port-forward Prometheus + Loki from your current k8s context
run-portfwd:
	@echo "Port-forwarding Prometheus → localhost:9090 and Loki → localhost:3100"
	@kubectl port-forward -n monitoring pod/$$(kubectl get pod -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}') 9090:9090 &
	@kubectl port-forward -n loki svc/loki 3100:3100 &
	@echo "Done. PIDs saved. Run 'make stop-portfwd' to kill."

## stop: kill all local obsidianstack processes
stop:
	@pkill -f obsidianstack-agent  2>/dev/null && echo "agent stopped"  || echo "agent not running"
	@pkill -f obsidianstack-server 2>/dev/null && echo "server stopped" || echo "server not running"

## ── Code quality ────────────────────────────────────────────────────────────

proto: ## Regenerate Go code from snapshot.proto
	buf generate

lint: ## Run golangci-lint
	golangci-lint run ./...

test: ## Run all tests with race detector
	$(GOBIN) test -race -count=1 ./...

test-short: ## Run tests, skipping slow integration tests
	$(GOBIN) test -short -race -count=1 ./...

tidy: ## Update go.sum
	$(GOBIN) mod tidy

## ── Docker ──────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -rf $(BIN)

docker-agent: ## Build agent Docker image
	docker build -f agent/Dockerfile -t obsidianstack/agent:dev .

docker-server: ## Build server Docker image
	docker build -f server/Dockerfile -t obsidianstack/server:dev .

up: ## Start full stack with docker-compose
	docker compose up --build

down: ## Stop docker-compose stack
	docker compose down

## ── Help ────────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*## "}{printf "  %-20s %s\n", $$1, $$2}'
