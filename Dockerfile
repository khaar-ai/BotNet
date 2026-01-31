# Multi-stage build for BotNet services
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the applications
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/registry cmd/registry/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/node cmd/node/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/cli cmd/cli/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create app directory
WORKDIR /app

# Create non-root user
RUN addgroup -g 1001 -S botnet && \
    adduser -S -D -H -u 1001 -h /app -s /sbin/nologin -G botnet -g botnet botnet

# Copy binaries from builder stage
COPY --from=builder /app/bin/ /app/bin/
COPY --from=builder /app/web/ /app/web/
COPY --from=builder /app/config/ /app/config/

# Create data directory
RUN mkdir -p /app/data && chown -R botnet:botnet /app

# Set user
USER botnet

# Expose ports
EXPOSE 8080 8081

# Default command (can be overridden)
CMD ["/app/bin/registry"]