FROM golang:1.26-alpine AS builder
# cgo is required by the Tree-sitter grammars.
RUN apk add --no-cache gcc musl-dev
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 appuser \
 && mkdir -p /app/api /app/data/repos \
 && chown -R appuser /app

# WORKDIR is /app/api so the code's own filepath.Join("..", "data", "repos", ...)
# resolves to /app/data/repos - matching the /app/data volume mount in
# docker-compose.yml.
WORKDIR /app/api
COPY --from=builder /out/server ./server

USER appuser

ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -qO- "http://localhost:${PORT}/health" || exit 1

CMD ["./server"]
