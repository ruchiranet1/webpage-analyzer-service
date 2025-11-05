# Webpage Analyzer Service

![Go Version] : 1.24.2

Analyze any web page and provide the detailed breakdown of its content and structure.


## 🚀 Getting Started

### Prerequisites

* [Go](https://go.dev/doc/install) (version 1.24.2 or later)

### How to Run

1.  **Clone the repository** (or just have the files in a folder):
    ```sh
    # If you've pushed it to Git
    git clone [https://github.com/ruchiranet1/webpage-analyzer-service.git](https://github.com/ruchiranet1/webpage-analyzer-service.git)
    cd webpage-analyzer-service
    ```

2.  **Install dependencies:**
    (This will download `gin` and create the `go.sum` file)
    ```sh
    go mod tidy
    ```

3.  **Run the server:**
    ```sh
    go run ./cmd/api/main.go
    ```

The server is now running on `http://localhost:8080`.

##  API Endpoints

### Health Check

The one, the only, the glorious. This endpoint confirms the server is alive and breathing.

* **Endpoint:** `GET /health`
* **Success Response (200 OK):**
    ```json
    {
      "status": "ok"
    }
    ```