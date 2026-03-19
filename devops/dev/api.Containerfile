FROM golang:1.26-alpine

WORKDIR /app

# Copy workspace files
COPY backend/go.work ./
COPY backend/go.work.sum ./

# Copy module files
COPY backend/services/api/go.mod backend/services/api/go.sum ./services/api/
COPY backend/shared/go.mod ./shared/
COPY backend/shared/messaging/go.mod ./shared/messaging/
COPY backend/shared/telemetry/go.mod ./shared/telemetry/

# Download dependencies
RUN go work sync && go mod download

# Copy source code
COPY backend/services/api/ ./services/api/
COPY backend/shared/ ./shared/

# Build the binary
RUN go work sync && go build -o /tmp/ace-api ./services/api/cmd/main.go

# Install air for hot reloading
RUN go install github.com/air-verse/air@latest

EXPOSE 8080

CMD ["air", "-c", "air.toml"]