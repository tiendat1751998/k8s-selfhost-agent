.PHONY: build test lint run clean docker-build docker-up docker-down migrate help

# Variables
BINARY_DIR := bin
SERVER_BINARY := $(BINARY_DIR)/server
AGENT_BINARY := $(BINARY_DIR)/agent-runner
GO := go
GOFLAGS := -v
LDFLAGS := -s -w
DOCKER_COMPOSE := docker compose -f deployments/docker/docker-compose.yml

# Default target
.DEFAULT_GOAL := help

## build: Build all binaries
build:
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(SERVER_BINARY) ./cmd/server
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(AGENT_BINARY) ./cmd/agent-runner

## test: Run all tests
test:
	$(GO) test ./... -v -count=1 -race -coverprofile=coverage.out

## test-short: Run short tests only
test-short:
	$(GO) test ./... -v -short -count=1

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format code
fmt:
	$(GO) fmt ./...
	goimports -w .

## run: Run the server locally
run:
	$(GO) run ./cmd/server

## run-agent: Run the agent runner locally
run-agent:
	$(GO) run ./cmd/agent-runner

## clean: Remove build artifacts
clean:
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

## docker-build: Build Docker image
docker-build:
	docker build -t k8sselfhost:latest -f deployments/docker/Dockerfile .

## docker-up: Start all services with Docker Compose
docker-up:
	$(DOCKER_COMPOSE) up -d

## docker-down: Stop all Docker Compose services
docker-down:
	$(DOCKER_COMPOSE) down

## docker-logs: View Docker Compose logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f

## migrate: Run database migrations
migrate:
	@echo "Running migrations..."
	@for f in migrations/*.sql; do \
		echo "Applying $$f..."; \
		PGPASSWORD=postgres psql -h localhost -U postgres -d k8sselfhost -f "$$f"; \
	done

## coverage: Generate HTML coverage report
coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html

## tidy: Tidy go modules
tidy:
	$(GO) mod tidy

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
