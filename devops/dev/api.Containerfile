FROM golang:1.26-alpine

WORKDIR /app

# Copy workspace files
COPY backend/go.work ./
COPY backend/services/api/go.mod backend/services/api/go.sum ./services/api/
COPY backend/shared/go.mod ./shared/

# Download dependencies
RUN go mod download

# Copy source code
COPY backend/services/api/ ./services/api/
COPY backend/shared/ ./shared/

RUN go build -o /tmp/ace-api ./services/api/cmd/main.go

EXPOSE 8080

CMD ["/tmp/ace-api"]