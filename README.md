# supervisord-metrics-exporter

Prometheus exporter for Supervisord. It exposes process state, uptime, and scrape health metrics for a running Supervisor instance.

## Table of Contents

- [Build the Go binary](#build-the-go-binary)
- [Run the exporter locally](#run-the-exporter-locally)
- [Build and run with Docker](#build-and-run-with-docker)
- [Docker smoke test](#docker-smoke-test)
- [Expected metrics](#expected-metrics)
- [License](#license)

## Build the Go binary

Requirements:

- Go 1.22 or newer

Build the executable from the repository root:

```sh
go build -o supervisord-metrics-exporter .
```

Run the binary with the default flags:

```sh
./supervisord-metrics-exporter -h
```

## Run the exporter locally

The exporter accepts the following flags:

- `-supervisor.url` — Supervisor XML-RPC URL (default: `http://localhost:9001/RPC2`)
- `-supervisor.username` — optional HTTP basic auth username
- `-supervisor.password` — optional HTTP basic auth password
- `-web.listen.address` — HTTP bind address for metrics (default: `:9002`)
- `-web.metrics.endpoint` — metrics path (default: `/metrics`)
- `-version` — print the application version and exit

Example:

```sh
./supervisord-metrics-exporter \
  -supervisor.url http://127.0.0.1:9001/RPC2 \
  -supervisor.username exporter \
  -supervisor.password exporter-password \
  -web.listen.address :9002 \
  -web.metrics.endpoint /metrics
```

Then query the metrics endpoint:

```sh
curl http://localhost:9002/metrics
```

## Build and run with Docker

Build the image:

```sh
docker build -t supervisord-metrics-exporter:local -f docker/Dockerfile .
```

Run the container with the sample Supervisor configuration:

```sh
docker run -d \
  --name supervisord-metrics-exporter \
  -p 9001:9001 \
  -p 9002:9002 \
  supervisord-metrics-exporter:local
```

The sample Supervisor setup inside the container uses the following credentials:

```text
username: exporter
password: exporter-password
```

## Docker smoke test

Check the Supervisor-managed processes:

```sh
docker exec supervisord-metrics-exporter supervisorctl status
```

Check the exporter metrics on container:

```sh
curl http://localhost:9002/metrics
```

Inspect container logs if needed:

```sh
docker logs supervisord-metrics-exporter
```

Stop and remove the container when you are done:

```sh
docker stop supervisord-metrics-exporter
docker rm supervisord-metrics-exporter
```

## Expected metrics

Useful lines to confirm in the output:

```text
supervisord_up 1
supervisor_process_state{process_name="dummy-api",state="RUNNING",exit_status="0"} 1
supervisor_process_state{process_name="dummy-worker",state="RUNNING",exit_status="0"} 1
supervisor_process_state{process_name="supervisord-metrics-exporter",state="RUNNING",exit_status="0"} 1
supervisor_process_uptime{process_name="dummy-api"}
supervisord_scrape_duration_seconds
```

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for the full text.
