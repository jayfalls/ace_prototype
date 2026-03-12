# Build stage
FROM golang:1.26-alpine AS builder

# Install air for hot reload
RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /ace-api .

# Production stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /ace-api .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./ace-api"]
