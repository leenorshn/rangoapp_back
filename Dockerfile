# Multi-stage build for Cloud Run deployment
# Stage 1: Build the application
# Using golang:alpine for latest stable version
# go mod tidy will ensure compatibility during build
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Ensure go.mod is up to date with all source files
RUN go mod tidy

# Build the application
# CGO_ENABLED=0 for static binary, useful for distroless images
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o /app/server \
    ./server.go

# Stage 2: Create minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /app/server /server

# Use non-root user (distroless images already run as nonroot)
USER nonroot:nonroot

# Expose port (Cloud Run will set PORT env var automatically)
# Cloud Run uses the PORT environment variable, which is already handled in server.go
EXPOSE 8080

# Run the server
# Note: Cloud Run will automatically set the PORT environment variable
ENTRYPOINT ["/server"]

