# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /app/build/anyfeed ./cmd/anyfeed

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary
COPY --from=builder /app/build/anyfeed /app/anyfeed

# Create directories for data and config
RUN mkdir -p /app/data /app/configs

EXPOSE 8080 2525

VOLUME ["/app/data", "/app/configs"]

ENTRYPOINT ["/app/anyfeed"]
CMD ["--config", "/app/configs/config.yaml"]
