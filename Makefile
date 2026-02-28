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
        docker-agent docker-server docker-ui docker-build docker-push \
        helm-lint helm-template helm-install helm-upgrade-cluster \
        rollout-restart release up down help

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
##
## Workflow for releasing a new version:
##
##   1. Build and test locally:
##        make docker-build VERSION=v0.1.1
##        docker compose up  (or use make helm-upgrade-cluster to test in-cluster)
##
##   2. When happy, push to Docker Hub:
##        make docker-push VERSION=v0.1.1 DOCKER_USER=marocz
##
##   3. Update tag in deploy/cluster/values.yaml, then push branch and open PR:
##        git push origin HEAD
##        gh pr create
##
##   4. After PR is merged to main, tag the release — CI builds official
##      multi-arch images (amd64 + arm64) and creates a GitHub Release:
##        git checkout main && git pull
##        git tag v0.1.1 && git push origin v0.1.1
##
# Override on the command line:  make docker-push DOCKER_USER=yourname VERSION=v0.1.0
DOCKER_USER ?= marocz
VERSION     ?= dev

clean: ## Remove build artifacts
	rm -rf $(BIN)

docker-agent: ## Build agent image locally (single-arch, fast)
	docker build -f agent/Dockerfile -t $(DOCKER_USER)/obsidianstack-agent:$(VERSION) .

docker-server: ## Build server image locally (single-arch, fast)
	docker build -f server/Dockerfile -t $(DOCKER_USER)/obsidianstack-server:$(VERSION) .

docker-ui: ## Build UI image locally (single-arch, fast)
	docker build -f ui/Dockerfile -t $(DOCKER_USER)/obsidianstack-ui:$(VERSION) ./ui

docker-build: docker-agent docker-server docker-ui ## Build all three images locally (single-arch)
	@echo ""
	@echo "Images built:"
	@echo "  $(DOCKER_USER)/obsidianstack-agent:$(VERSION)"
	@echo "  $(DOCKER_USER)/obsidianstack-server:$(VERSION)"
	@echo "  $(DOCKER_USER)/obsidianstack-ui:$(VERSION)"
	@echo ""
	@echo "Next: test locally, then push with: make docker-push VERSION=$(VERSION)"

docker-push: ## Build multi-arch + push all three images to Docker Hub (requires: docker login)
	@if [ "$(VERSION)" = "dev" ]; then \
	  echo "ERROR: set a real version — e.g.  make docker-push VERSION=v0.1.0"; exit 1; \
	fi
	docker buildx build --platform linux/amd64,linux/arm64 \
	  -f agent/Dockerfile \
	  -t $(DOCKER_USER)/obsidianstack-agent:$(VERSION) \
	  --push .
	docker buildx build --platform linux/amd64,linux/arm64 \
	  -f server/Dockerfile \
	  -t $(DOCKER_USER)/obsidianstack-server:$(VERSION) \
	  --push .
	docker buildx build --platform linux/amd64,linux/arm64 \
	  -f ui/Dockerfile \
	  -t $(DOCKER_USER)/obsidianstack-ui:$(VERSION) \
	  --push ./ui
	@echo ""
	@echo "Pushed $(VERSION) to Docker Hub."
	@echo "Update deploy/cluster/values.yaml image tags, then open a PR."

release: test docker-build ## Run all tests then build images — confirms everything is green before pushing
	@echo ""
	@echo "All tests passed and images built for VERSION=$(VERSION)."
	@echo ""
	@echo "Release checklist:"
	@echo "  1. Push images to Docker Hub:"
	@echo "       make docker-push VERSION=$(VERSION)"
	@echo "  2. Update image tags in deploy/cluster/values.yaml"
	@echo "  3. Deploy to cluster and verify:"
	@echo "       make helm-upgrade-cluster"
	@echo "       make rollout-restart"
	@echo "  4. Commit + push feature branch, open PR, merge to main"
	@echo "  5. Tag to trigger official CI release:"
	@echo "       git tag $(VERSION) && git push origin $(VERSION)"

up: ## Start full stack with docker-compose
	docker compose up --build

down: ## Stop docker-compose stack
	docker compose down

## ── Helm ────────────────────────────────────────────────────────────────────

HELM_RELEASE        ?= obsidianstack
HELM_NS             ?= obsidianstack
HELM_CHART          := charts/obsidianstack
HELM_CLUSTER_VALUES ?= deploy/cluster/values.yaml

helm-lint: ## Lint the Helm chart
	helm lint $(HELM_CHART)

helm-template: ## Render Helm templates (dry-run, default values)
	helm template $(HELM_RELEASE) $(HELM_CHART) --namespace $(HELM_NS)

helm-template-cluster: ## Render Helm templates with cluster values (dry-run)
	helm template $(HELM_RELEASE) $(HELM_CHART) \
	  --namespace $(HELM_NS) \
	  --values $(HELM_CLUSTER_VALUES)

helm-install: ## Install / upgrade the chart with default values
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
	  --namespace $(HELM_NS) \
	  --create-namespace \
	  --values $(HELM_CHART)/values.yaml

helm-upgrade-cluster: ## Deploy to your current k8s context using deploy/cluster/values.yaml (+ values.local.yaml if present)
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
	  --namespace $(HELM_NS) \
	  --create-namespace \
	  --values $(HELM_CLUSTER_VALUES) \
	  $(shell test -f deploy/cluster/values.local.yaml && echo "--values deploy/cluster/values.local.yaml")

rollout-restart: ## Restart all ObsidianStack pods to pick up new images
	kubectl rollout restart deployment -n $(HELM_NS)
	@echo "Waiting for rollout..."
	kubectl rollout status deployment -n $(HELM_NS) --timeout=120s

helm-uninstall: ## Uninstall the release
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NS)

## ── Help ────────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*## "}{printf "  %-20s %s\n", $$1, $$2}'
