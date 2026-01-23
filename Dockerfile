# --- Stage 1: Builder ---
FROM golang:1.25.1-alpine AS builder

WORKDIR /build

# Installiere git (falls Dependencies git benötigen)
RUN apk add --no-cache git

# Dependencies cachen
COPY go.mod go.sum ./
RUN go mod download

# Source Code kopieren
COPY . .

# Binary bauen
# CGO_ENABLED=0 -> Statisch gelinkte Binary (läuft überall)
# -ldflags="-s -w" -> Entfernt Debug-Infos (kleineres Binary)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o refinery \
    ./cmd/refinery/main.go

# --- Stage 2: Runtime ---
FROM alpine:latest

# Sicherheit: Non-Root User erstellen
RUN addgroup -S refinery && adduser -S refinery -G refinery

WORKDIR /app

# Notwendige Runtime-Pakete (Zertifikate für HTTPS, Zeitzonen)
RUN apk --no-cache add ca-certificates tzdata

# Binary aus Stage 1 kopieren
COPY --from=builder /build/refinery .

# Statische Assets und Migrationen kopieren
COPY --from=builder /build/web ./web
COPY --from=builder /build/migrations ./migrations

# Config kopieren (Optional: In K8s oft via ConfigMap gemountet)
# COPY config.yaml .

# Berechtigungen setzen
RUN chown -R refinery:refinery /app

# Auf sicheren User wechseln
USER refinery

# Ports dokumentieren
EXPOSE 8080 9090

# Healthcheck definieren
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./refinery"]
