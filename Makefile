# Makefile for the Web Page Analyzer service

# Define the binary name and output directory
BINARY_NAME=server
IMAGE_NAME=webpage-analyzer-service
BINARY_DIR=.\bin
COVERAGE_FILE=coverage.out

# --- Build Tasks ---

# build: Compiles the Go application
.PHONY: build
build:
	@echo "Building application..."
	@go build -o $(BINARY_DIR)\$(BINARY_NAME) ./cmd/server/main.go
	@echo "Build complete: $(BINARY_DIR)\$(BINARY_NAME)"

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

# coverage: Runs tests and generates a code coverage report, filtering out 0.0% functions
.PHONY: coverage
coverage:
	@echo "Running tests and generating coverage report..."
	@go test ./... -coverprofile="$(COVERAGE_FILE)"


# tidy: Tidies the go.mod and go.sum files
.PHONY: tidy
tidy:
	@echo "Tidying modules..."
	@go mod tidy

# clean: Removes the build directory (Windows-compatible) and coverage files
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@if exist $(BINARY_DIR) ( RMDIR /S /Q $(BINARY_DIR) ) else ( echo "No binary directory to clean." )
	@if exist $(COVERAGE_FILE) ( DEL /F /Q $(COVERAGE_FILE) ) else ( echo "No coverage file to clean." )

# --- Docker Tasks ---

# docker-build: Builds the Docker image
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(IMAGE_NAME):latest .

# docker-run: Runs the Docker container
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 -e "LOGGER_LEVEL=debug" --rm $(IMAGE_NAME):latest

# docker-push:
# Usage: make docker-push REPO=docker-repo-name/webpage-analyzer-service
.PHONY: docker-push
docker-push:
	@echo "Tagging image $(IMAGE_NAME):latest as $(REPO):latest..."
	@docker tag $(IMAGE_NAME):latest $(REPO):latest
	@echo "Pushing $(REPO):latest..."
	@docker push $(REPO):latest
	@echo "Push complete."

.DEFAULT_GOAL := build