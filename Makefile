# ObsidianStack Makefile

BINARY_AGENT  := obsidianstack-agent
BINARY_SERVER := obsidianstack-server
MODULE        := github.com/obsidianstack/obsidianstack

AGENT_MAIN    := ./agent/cmd/agent
SERVER_MAIN   := ./server/cmd/server

.PHONY: all build build-agent build-server proto lint test clean docker-agent docker-server

all: proto build

## build: compile both binaries
build: build-agent build-server

build-agent:
	go build -o bin/$(BINARY_AGENT) $(AGENT_MAIN)

build-server:
	go build -o bin/$(BINARY_SERVER) $(SERVER_MAIN)

## proto: regenerate Go code from snapshot.proto
proto:
	buf generate

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## test: run all tests with race detector
test:
	go test -race -count=1 ./...

## test-short: run tests without slow integration tests
test-short:
	go test -short -race -count=1 ./...

## tidy: update go.sum
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin/

## docker-agent: build agent Docker image
docker-agent:
	docker build -f agent/Dockerfile -t obsidianstack/agent:dev .

## docker-server: build server Docker image
docker-server:
	docker build -f server/Dockerfile -t obsidianstack/server:dev .

## up: start full stack with docker-compose
up:
	docker compose up --build

## down: stop docker-compose stack
down:
	docker compose down
