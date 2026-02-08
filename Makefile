.PHONY: build clean test install run validate docker-build

# Build variables
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILDTIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILDTIME)"

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Binary name
BINARY_NAME := ora2csv
BUILD_DIR := bin

# All platforms to build for
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

default: build

## build: Build the binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ora2csv
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

## build-all: Build binaries for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@$(foreach platform,$(PLATFORMS), \
		echo "Building $(platform)..."; \
		GOOS=$(word 1,$(subst /, ,$(platform))) \
		GOARCH=$(word 2,$(subst /, ,$(platform))) \
		$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-$(platform)$(if $(findstring windows,$(platform)),.exe,) ./cmd/ora2csv; \
	)
	@echo "Build complete"

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@$(GOCMD) clean
	@echo "Clean complete"

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies ready"

## install: Install the binary to $GOPATH/bin or $HOME/go/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	@$(GOCMD) install $(LDFLAGS) ./cmd/ora2csv
	@echo "Installed $(BINARY_NAME)"

## run: Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) export --verbose

## validate: Validate configuration without connecting to database
validate: build
	./$(BUILD_DIR)/$(BINARY_NAME) validate

## validate-full: Validate configuration with database connection test
validate-full: build
	./$(BUILD_DIR)/$(BINARY_NAME) validate --test-connection

## fmt: Format Go source code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...
	@echo "Code formatted"

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		GOCACHE=$${GOCACHE:-/tmp/go-build-cache} \
		GOLANGCI_LINT_CACHE=$${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache} \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/"; \
	fi

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t ora2csv:$(VERSION) .
	docker tag ora2csv:$(VERSION) ora2csv:latest
	@echo "Docker image built: ora2csv:$(VERSION)"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
