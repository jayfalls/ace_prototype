FROM golang:1.26-alpine

WORKDIR /app

COPY backend/services/api/go.mod backend/services/api/go.sum ./
RUN go mod download
RUN go install github.com/air-verse/air@latest

EXPOSE 8080

CMD ["air", "-c", "air.toml"]