# Webpage Analyzer Service

Analyze any web page and provide the detailed breakdown of its content and structure.

Analyzes web page URL and returns:
- HTML version
- Page title
- Heading counts (h1–h6)
- Internal / External / Inaccessible links
- Login form is there or not


## 🚀 Getting Started

### Prerequisites

* [Go](https://go.dev/doc/install) (version 1.24.2 or later)

### How to Run

1.  **Clone the repository** (or just have the files in a folder):
    ```sh
    # If you've pushed it to Git
    git clone https://github.com/ruchiranet1/webpage-analyzer-service.git
    cd webpage-analyzer-service
    ```

2.  **Install dependencies:**
    (This will download `gin` and create the `go.sum` file)
    ```sh
    go mod tidy
    ```

3.  **Run the server:**
    ```sh
    go run ./cmd/api/main.go or 
    make run
    ```

The server is now running on `http://localhost:8080`.

##  API Endpoints

### 1. Health Check

This endpoint confirms the server is alive and healthy.

* **Endpoint:** `GET /health`
* **Success Response (200 OK):**
    ```json
    {
      "status": "ok"
    }
    ```

### 2. Analyser Post API

This endpoint confirms the server is alive and healthy.

* **Endpoint:** `POST /api/v1/analyzes`
* **Request:**
    ```bash
    curl -X \
    POST http://localhost:8080/api/v1/analyzes \
    -H \
    "Content-Type: application/json" \
    -d '{
        "requestId": "req-123",
        "userId": "user-1",
        "channel": "web",
        "email": "test@analyzer.com",
        "url": "https://analyzer.com"
    }'
    ```
* **Response:**
 ```json
    {
        "htmlVersion": "HTML5",
        "pageTitle": "Web Analyzer",
        "headings": { "h1": 1 },
        "links": { "internal": 0, "external": 0, "inaccessible": 0 },
        "hasLoginForm": false
    }
    ```
