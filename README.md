# ObsidianStack

> Open-source observability pipeline health monitor.

Track data drop %, recovery rate, throughput, latency, and pipeline strength across metrics, logs, and traces. Supports Kubernetes (OpenShift, AKS), VMs, and external stacks. Connects via mTLS, API key, or bearer token. Ships as a Go agent + server with a React dashboard and Helm chart.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

---

## What it does

ObsidianStack scrapes internal Prometheus metrics from your observability pipeline components (OTel Collector, Prometheus, Loki) and computes a **Pipeline Strength Score** (0–100) using:

| Factor | Weight | Description |
|--------|--------|-------------|
| Drop Rate | 40% | % of data dropped by exporters/processors |
| Latency | 30% | Export latency P95 vs baseline |
| Recovery Rate | 20% | % of backpressure that self-recovered |
| Uptime | 10% | Rolling scrape success rate |

Health states: **Healthy** ≥85 · **Degraded** 60–84 · **Critical** <60 · **Unknown**

---

## Architecture

```
Data Sources (k8s / VMs / external)
         │
         ▼
  obsidianstack-agent
  ├── Scrapers  (OTel Collector, Prometheus, Loki)
  ├── Compute Engine  (drop%, latency, strength score)
  └── gRPC Shipper  (mTLS / API key, buffer + retry)
         │  gRPC
         ▼
  obsidianstack-server
  ├── gRPC Receiver  (validates auth, stores snapshots)
  ├── REST API       (/api/v1/health, /pipelines, ...)
  └── WebSocket      (/ws/stream — live push)
         │  HTTP / WS
         ▼
  obsidianstack-ui  (React + Vite + Tailwind)
```

---

## Repository layout

```
obsidianstack/
├── agent/               # Go agent binary
│   ├── cmd/agent/
│   └── internal/
│       ├── config/      # YAML config loader + hot-reload
│       ├── scraper/     # OTel, Prometheus, Loki scrapers
│       ├── compute/     # strength score + delta engine
│       └── shipper/     # gRPC client with buffer + retry
├── server/              # Go server binary
│   ├── cmd/server/
│   └── internal/
│       ├── config/      # server config loader
│       ├── receiver/    # gRPC SnapshotService impl
│       ├── store/       # in-memory state + TTL
│       ├── auth/        # API key + mTLS middleware
│       ├── api/         # REST handlers
│       ├── ws/          # WebSocket hub
│       └── alerts/      # rule engine + webhooks
├── ui/                  # React dashboard (Phase 4)
├── proto/obsidian/v1/   # Protobuf schema
├── gen/obsidian/v1/     # Generated gRPC Go code
├── charts/helm/         # Helm chart (Phase 5)
├── deploy/              # k8s manifests + systemd units
├── config.example.yaml
├── Makefile
└── docker-compose.yaml
```

---

## Quickstart

### Prerequisites

- Go 1.21+
- Docker + Docker Compose (for the full stack)

### Run with Docker Compose

```bash
cp config.example.yaml config.yaml
# edit config.yaml — set your source endpoints and API keys
docker compose up
```

- Agent scrapes your pipeline components and ships to the server
- Server listens on `:50051` (gRPC) and `:8080` (HTTP)
- Dashboard at `http://localhost:8080`

### Build locally

```bash
make build-agent   # → bin/obsidianstack-agent
make build-server  # → bin/obsidianstack-server
```

### Run tests

```bash
make test          # all tests (requires Go 1.21+)
make test-short    # skip integration tests
```

---

## Configuration

Copy `config.example.yaml` and edit:

```yaml
agent:
  server_endpoint: "localhost:50051"
  scrape_interval: 30s

  sources:
    - id: "otel-prod"
      type: otelcol
      endpoint: "http://otelcol.monitoring.svc:8888/metrics"
      auth:
        mode: mtls
        cert_file: /etc/certs/client.crt
        key_file:  /etc/certs/client.key
        ca_file:   /etc/certs/ca.crt

    - id: "prometheus-prod"
      type: prometheus
      endpoint: "http://prometheus.monitoring.svc:9090/metrics"
      auth:
        mode: apikey
        header: X-API-Key
        key_env: PROM_API_KEY

server:
  grpc_port: 50051
  http_port: 8080
  auth:
    mode: apikey
    key_env: OBSIDIAN_SERVER_KEY
```

**Supported source types:** `otelcol` · `prometheus` · `loki`

**Auth modes:** `mtls` · `apikey` · `bearer` · `none`

---

## REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Overall health score + alert count |
| GET | `/api/v1/pipelines` | All pipelines with status |
| GET | `/api/v1/pipelines/{id}` | Single pipeline detail |
| GET | `/api/v1/signals` | Metrics / logs / traces breakdown |
| GET | `/api/v1/alerts` | Active alert list |
| GET | `/api/v1/certs` | Cert + key status per source |
| GET | `/api/v1/snapshot` | Full stack JSON dump |
| GET | `/metrics` | Prometheus self-metrics |
| WS  | `/ws/stream` | Live push stream (JSON, every 5s) |

---

## Development status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Foundation & Agent Core | ✅ Complete |
| 2 | Server Aggregator & REST API | 🔄 In progress |
| 3 | Security — TLS, Certs, API Keys | Planned |
| 4 | React UI Dashboard | Planned |
| 5 | Helm Chart & K8s Manifests | Planned |
| 6 | VM & External Source Support | Planned |
| 7 | Alerting & Webhook Integrations | Planned |
| 8 | Historical Storage & Trend Charts | Planned |

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
