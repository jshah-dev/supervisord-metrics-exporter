FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/supervisord-metrics-exporter .

FROM debian:bookworm-slim

LABEL org.opencontainers.image.title="supervisord-metrics-exporter"
LABEL org.opencontainers.image.description="Prometheus exporter for Supervisord"

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl supervisor \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/supervisord-metrics-exporter /usr/local/bin/supervisord-metrics-exporter
COPY docker/supervisord.conf /etc/supervisor/supervisord.conf
COPY docker/dummy-api.sh /usr/local/bin/dummy-api
COPY docker/dummy-worker.sh /usr/local/bin/dummy-worker

RUN chmod +x /usr/local/bin/dummy-api /usr/local/bin/dummy-worker

EXPOSE 9002 9001

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/supervisord.conf"]
