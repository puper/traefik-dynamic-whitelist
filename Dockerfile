FROM golang:1.26.3-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM alpine:3.22.4

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/gateway /usr/local/bin/gateway

RUN mkdir -p /data /traefik-dynamic && chown -R app:app /data /traefik-dynamic

USER app
EXPOSE 8080

ENV LISTEN_ADDR=:8080 \
    STATE_PATH=/data/state.json \
    TRAEFIK_DYNAMIC_PATH=/traefik-dynamic/whitelist.yml

ENTRYPOINT ["/usr/local/bin/gateway"]
