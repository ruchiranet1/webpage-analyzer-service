# --- Stage 1: Builder ---
# This stage builds the Go binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module files and download dependencies
# This is done first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# -o /app/server: Output the binary to /app/server
# -ldflags "-s -w": Strips debug symbols and info, making the binary smaller
# CGO_ENABLED=0: Disables CGO, which is crucial for a static binary
# GOOS=linux GOARCH=amd64: Ensures we build for a linux/amd64 (Alpine) environment
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/server -ldflags="-s -w" ./cmd/server/main.go

# --- Stage 2: Final ---
# This is the final, minimal production image
FROM alpine:latest

# We need ca-certificates for making HTTPS requests (e.g., fetching pages)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the compiled binary from the 'builder' stage
COPY --from=builder /app/server .

# Set the binary as executable
RUN chmod +x ./server

# Expose the port the application will run on
EXPOSE 8080

# Set the command to run the application
# This is equivalent to `docker run <image> ./server`
CMD ["./server"]