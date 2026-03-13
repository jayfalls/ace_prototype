# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY backend/services/api/go.mod backend/services/api/go.sum ./
RUN go mod download

# Copy source code from new location
COPY backend/services/api/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /ace-api ./cmd

# Production stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -u 1000 appuser

# Copy binary from builder
COPY --from=builder /ace-api .

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["./ace-api"]
