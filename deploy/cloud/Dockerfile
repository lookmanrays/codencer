FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out && \
    go build -ldflags "-X agent-bridge/internal/app.Version=v0.2.0-alpha" -o /out/codencer-cloudd ./cmd/codencer-cloudd && \
    go build -ldflags "-X agent-bridge/internal/app.Version=v0.2.0-alpha" -o /out/codencer-cloudctl ./cmd/codencer-cloudctl && \
    go build -ldflags "-X agent-bridge/internal/app.Version=v0.2.0-alpha" -o /out/codencer-cloudworkerd ./cmd/codencer-cloudworkerd && \
    go build -ldflags "-X agent-bridge/internal/app.Version=v0.2.0-alpha" -o /out/codencer-relayd ./cmd/codencer-relayd

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/codencer-cloudd /usr/local/bin/codencer-cloudd
COPY --from=builder /out/codencer-cloudctl /usr/local/bin/codencer-cloudctl
COPY --from=builder /out/codencer-cloudworkerd /usr/local/bin/codencer-cloudworkerd
COPY --from=builder /out/codencer-relayd /usr/local/bin/codencer-relayd

RUN mkdir -p /var/lib/codencer/cloud /var/lib/codencer/relay /etc/codencer/cloud /etc/codencer/relay

EXPOSE 8190

ENTRYPOINT ["/usr/local/bin/codencer-cloudd"]
CMD ["--config", "/etc/codencer/cloud/config.json"]
