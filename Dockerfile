FROM golang:1.24.2-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api cmd/api/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/bin/api /api
EXPOSE 8080
CMD ["/api"]