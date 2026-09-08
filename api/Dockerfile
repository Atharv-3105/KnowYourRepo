FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

# WORKDIR is /app/api so the code's own filepath.Join("..", "data", "repos", ...)
# resolves to /app/data/repos - matching the ./data:/app/data volume mount
# in docker-compose.yml.
WORKDIR /app/api
COPY --from=builder /out/server ./server

EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./server"]