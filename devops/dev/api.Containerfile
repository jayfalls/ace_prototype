# Build stage
FROM golang:1.26-alpine AS builder

# Install air for hot reload
RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY backend/services/api/go.mod backend/services/api/go.sum ./
RUN go mod download

# Copy source code from the new location
COPY backend/services/api/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /ace-api ./cmd

# Production stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates and copy Go from builder for hot reload rebuilds
RUN apk --no-cache add ca-certificates
COPY --from=builder /usr/local/go /usr/local/go

# Add Go to PATH
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOTOOLCHAIN=local

# Copy binary from builder
COPY --from=builder /ace-api .
COPY --from=builder /go/bin/air /usr/local/bin/air
COPY --from=builder /app/air.toml .

# Create tmp directory for air and change ownership
RUN mkdir -p /app/tmp

# Expose port
EXPOSE 8080

# Run air for hot reload in development
# Running as root to allow writing to mounted volumes
CMD ["air", "-c", "air.toml"]
