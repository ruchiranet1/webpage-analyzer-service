# Makefile for the Web Page Analyzer service

# Define the binary name and output directory
BINARY_NAME=server
BINARY_DIR=./bin

# --- Build Tasks ---

# build: Compiles the Go application
.PHONY: build
build:
	@echo "Building application..."
	@go build -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/server/main.go
	@echo "Build complete: $(BINARY_DIR)/$(BINARY_NAME)"

# run: Runs the application using go run (for development)
.PHONY: run
run:
	@echo "Starting application (dev)..."
	@go run ./cmd/server/main.go

# test: Runs all Go tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./... -v

# tidy: Tidies the go.mod and go.sum files
.PHONY: tidy
tidy:
	@echo "Tidying modules..."
	@go mod tidy

# clean: Removes the build directory (Windows-compatible)
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@if exist $(BINARY_DIR) ( RMDIR /S /Q $(BINARY_DIR) ) else ( echo "No build artifacts to clean." )

# --- Docker Tasks ---

# docker-build: Builds the Docker image
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t webpage-analyzer-service:latest .

# docker-run: Runs the Docker container
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 -e "LOGGER_LEVEL=debug" --rm webpage-analyzer-service:latest

# docker-push: (Placeholder) Tag and push the image to a registry
.PHONY: docker-push
docker-push:
	@echo "Tag and push logic would go here (e.g., docker push your-registry/webpage-analyzer-service)"

.DEFAULT_GOAL := build