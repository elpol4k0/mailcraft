# ── Stage 1: Build Frontend ────────────────────────────────────────────────
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ── Stage 2: Build Go Binary ───────────────────────────────────────────────
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Copy built frontend assets into the ui/ directory
COPY --from=frontend /app/web/../ui/ ./ui/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X mailcraft/internal/config.Version=$(cat /dev/null || echo dev)" \
    -o mailcraft \
    ./cmd/mailcraft

# ── Stage 3: Minimal final image ──────────────────────────────────────────
FROM scratch
COPY --from=builder /app/mailcraft /mailcraft
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 1025 8025

ENTRYPOINT ["/mailcraft"]
