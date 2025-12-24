.PHONY: all build clean test lint run dev-up dev-down help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary names
GATEWAY_BINARY=bin/gateway
SCHEDULER_BINARY=bin/scheduler
SESSION_MANAGER_BINARY=bin/session-manager
SANDBOX_AGENT_BINARY=bin/sandbox-agent

# Build flags
LDFLAGS=-ldflags "-s -w"

all: build

## build: Build all binaries
build: build-gateway build-scheduler build-session-manager build-sandbox-agent

build-gateway:
	$(GOBUILD) $(LDFLAGS) -o $(GATEWAY_BINARY) ./cmd/gateway

build-scheduler:
	$(GOBUILD) $(LDFLAGS) -o $(SCHEDULER_BINARY) ./cmd/scheduler

build-session-manager:
	$(GOBUILD) $(LDFLAGS) -o $(SESSION_MANAGER_BINARY) ./cmd/session-manager

build-sandbox-agent:
	$(GOBUILD) $(LDFLAGS) -o $(SANDBOX_AGENT_BINARY) ./cmd/sandbox-agent

## clean: Clean build files
clean:
	$(GOCLEAN)
	rm -rf bin/

## test: Run tests
test:
	$(GOTEST) -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## lint: Run linter
lint:
	$(GOLINT) run ./...

## deps: Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## install-tools: Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## dev-up: Start development environment
dev-up:
	docker compose up -d

## dev-down: Stop development environment
dev-down:
	docker compose down

## dev-logs: Show development environment logs
dev-logs:
	docker compose logs -f

## run-gateway: Run gateway service locally
run-gateway:
	$(GOCMD) run ./cmd/gateway

## run-scheduler: Run scheduler service locally
run-scheduler:
	$(GOCMD) run ./cmd/scheduler

## run-session-manager: Run session manager service locally
run-session-manager:
	$(GOCMD) run ./cmd/session-manager

## proto: Generate protobuf code
proto:
	protoc --go_out=. --go-grpc_out=. api/proto/*.proto

## docker-build: Build Docker images
docker-build:
	docker build -t cloud-sandbox/gateway:latest -f deploy/docker/Dockerfile.gateway .
	docker build -t cloud-sandbox/scheduler:latest -f deploy/docker/Dockerfile.scheduler .
	docker build -t cloud-sandbox/session-manager:latest -f deploy/docker/Dockerfile.session-manager .

## test-e2e: Run end-to-end tests (services must be running)
test-e2e:
	./tests/e2e/run_e2e_tests.sh

## test-e2e-python: Run Python SDK e2e tests
test-e2e-python:
	cd sdk/python && pip install -e . && pytest ../tests/e2e/test_e2e_python.py -v

## test-sdk: Run Python SDK unit tests
test-sdk:
	cd sdk/python && pip install -e ".[dev]" && pytest tests/ -v

## run-all: Start all services for development
run-all:
	@echo "Starting all services..."
	@$(GOCMD) run ./cmd/scheduler &
	@$(GOCMD) run ./cmd/session-manager &
	@sleep 2
	@$(GOCMD) run ./cmd/gateway

## k8s-dev: Deploy to Kubernetes (dev)
k8s-dev:
	kubectl apply -k deploy/k8s/overlays/dev

## k8s-prod: Deploy to Kubernetes (prod)
k8s-prod:
	kubectl apply -k deploy/k8s/overlays/prod

## k8s-delete-dev: Delete dev deployment
k8s-delete-dev:
	kubectl delete -k deploy/k8s/overlays/dev

## k8s-delete-prod: Delete prod deployment
k8s-delete-prod:
	kubectl delete -k deploy/k8s/overlays/prod

## help: Show this help
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
