# --- Stage 1: Builder ---
FROM golang:1.24-alpine AS builder

# Install ca-certificates which we will need in the final stage
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/server -ldflags="-s -w" ./cmd/server/main.go

# --- Stage 2: Final ---
FROM scratch

# 'scratch' is empty, so we must copy essential files from the builder
WORKDIR /app

# Copy the CA certificates from the builder stage for making HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compiled binary
COPY --from=builder /app/server .

# Copy the config file. This makes the image self-contained.
COPY config.properties .

# Expose the port
EXPOSE 8080

# Run the server
CMD ["./server"]