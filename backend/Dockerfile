# ==============================================================================
# Stage 1: Builder (Golang Alpine)
# ==============================================================================
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

# Cache de dependências do Go
COPY go.mod go.sum ./
RUN go mod download

# Copia código fonte
COPY . .

# Compilação estática dos binários sem CGO e com strip de símbolos de debug
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/api ./cmd/api

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/migrate ./cmd/migrate

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/bootstrap-admin ./cmd/bootstrap-admin

# ==============================================================================
# Stage 2: Runtime Mínimo de Alta Segurança (< 25MB)
# ==============================================================================
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/migrate /app/migrate
COPY --from=builder /app/bin/bootstrap-admin /app/bootstrap-admin
COPY --from=builder /app/migrations /app/migrations

RUN chown -R appuser:appgroup /app

USER appuser

ENV TZ=America/Sao_Paulo \
    PORT=8080 \
    GIN_MODE=release

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8080/ready || exit 1

CMD ["/app/api"]
