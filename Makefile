# Makefile for webpage-analyzer-service

.PHONY: test coverage run

test:
	@go test -v -cover ./...

coverage:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

run:
	@gin.SetMode(gin.ReleaseMode) && go run cmd/api/main.go