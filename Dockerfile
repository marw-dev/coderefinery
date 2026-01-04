# Build Stage
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Statisch gelinktes Binary bauen
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o refinery cmd/refinery/main.go

# Runtime Stage (Minimal)
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/refinery .
COPY --from=builder /app/start.sh .

# Ports und Volumes
EXPOSE 8080
VOLUME /data

# Default Command
CMD ["./refinery", "serve", "/data"]
