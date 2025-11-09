# Web Page Analyzer Service

A high-performance web page analyzer to analyze and provides a detailed breakdown as per following.

It analyzes a web page URL and returns:
* HTML version
* Page title
* Heading counts (h1–h6)
* Internal, external, and inaccessible link counts
* Whether a login form is present

## ✨ Features

* **Dual API Endpoints**: Synchronous (`/api/v1/analyzes`) for immediate results and Asynchronous (`/api/v1/analyzes/async`) for "fire-and-forget" requests.
* **Modern Architecture**: Built using Clean Architecture to decouple business logic from infrastructure.
* **Robust & Resilient**: Uses a Circuit Breaker for fetching and a concurrent Worker Pool for fast, safe link checking.
* **Production-Ready**: Includes authentication (JWT), rate limiting, and idempotency.
* **Fully Observable**: Exposes structured `slog` logs, Prometheus metrics (`/metrics`), and `pprof` profiling (`/debug`).
* **Configurable**: All settings are managed externally via a `config.properties` file (viper).

## 🛠️ Tech Stack

* **Framework**: `chi` (v5) for lightweight, high-performance routing.
* **Configuration**: `viper`
* **Logging**: `slog` (structured JSON logging)
* **Concurrency**: Goroutines, Channels, and WaitGroups (CSP)
* **Authentication**: `golang-jwt/jwt` (v5)
* **Resiliency**: `sony/gobreaker` (Circuit Breaker)
* **Observability**: `prometheus/client_golang`

## 🚀 Getting Started

### Prerequisites

* Go (version 1.24 or higher)
* Make (optional, for convenience)
* Docker (optional, for containerized running)

### 1. Configuration

The application is configured using the `config.properties` file. Before running, you must create this file in the root of the project.

2. How to Run
Option A: Run Locally (with make)

This is the easiest way to run the application for development.

```bash
# 1. Install dependencies
make tidy

# 2. Run the server
make run
```

Option B: Run Locally
```bash
# 1. Install dependencies
go mod tidy

# 2. Run the server
go run ./cmd/server/main.go
```

Option C: Run with Docker (Production-Style)
This builds the multi-stage Dockerfile and runs the production-ready container.

```bash
# 1. Build the Docker image
make docker-build

# 2. Run the container
make docker-run
```

The server is now running on http://localhost:8080.

📡 API Endpoints
All /api/v1 routes are protected by JWT authentication.

Authentication
First, set this valid JWT in your shell environment. This token is signed with the default jwt.secret from your config.properties.

```PowerShell
PowerShell

$TOKEN="generated_token"
```

Bash (Linux/macOS):
```bash
TOKEN="generated_token"
```

1. Synchronous Analysis
This endpoint sends a request, waits for the full analysis, and returns the JSON result.

Endpoint: POST /api/v1/analyzes

Request (PowerShell):

```PowerShell
curl -X POST http://localhost:8080/api/v1/analyzes `
-H "Authorization: Bearer $TOKEN" `
-H "Content-Type: application/json" `
-H "Idempotency-Key: $(New-Guid)" `
-d '{"requestId":"demo-sync-1","email":"test@example.com","url":"[https://httpbin.org/html](https://httpbin.org/html)"}'
```

Success Response (200 OK):
```json
{
    "html_version": "HTML 5",
    "title": "HTML 5 Test Page",
    "headings": {
        "h1": 1,
        "h2": 1,
        "h3": 1,
        "h4": 1,
        "h5": 1,
        "h6": 1
    },
    "links": {
        "internal": 1,
        "external": 0,
        "inaccessible": 0
    },
    "has_login_form": false
}
```

2. Asynchronous Analysis
This endpoint sends a request and immediately receives a 202 Accepted response. The analysis is processed in the background (visible in server logs).

Endpoint: POST /api/v1/analyzes/async

Request 
```PowerShell
PowerShell

curl -X POST http://localhost:8080/api/v1/analyzes/async `
-H "Authorization: Bearer $TOKEN" `
-H "Content-Type: application/json" `
-d '{"requestId":"demo-async-1","email":"test-async@example.com","url":"https://go.dev"}'
```
Success Response (202 Accepted):
JSON
```json
{
    "message": "Analysis request accepted and is being processed.",
    "requestId": "demo-async-1"
}
```

3. Health & Observability
These endpoints are not authenticated and are used for monitoring.

Health Check: GET /health

```bash

curl http://localhost:8080/health
# {"status":"ok"}
```
Prometheus Metrics: GET /metrics

```bash
curl http://localhost:8080/metrics
```

Go Profiling (pprof):
```bash
http://localhost:8080/debug/pprof/ (index)

http://localhost:8080/debug/pprof/goroutine?debug=1 (all goroutines)
```

