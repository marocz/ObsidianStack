# ObsidianStack

> Open-source observability pipeline health monitor.

Know instantly whether your metrics, logs, and traces are flowing — or silently dropping. ObsidianStack scrapes your pipeline components (OTel Collector, Prometheus, Loki, Fluent Bit), computes a **Pipeline Strength Score**, and surfaces drop rates, queue pressure, export failures, and recovery rates in a live dashboard.

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

---

## What it does

ObsidianStack scrapes internal metrics from your pipeline components and computes a **Pipeline Strength Score** (0–100):

| Factor | Weight | Description |
|--------|--------|-------------|
| Drop Rate | 40% | % of data dropped by exporters / processors |
| Latency | 30% | Export latency P95 vs your baseline |
| Recovery Rate | 20% | % of backpressure that self-recovered |
| Uptime | 10% | Rolling scrape success rate |

**Health states:** `healthy` ≥85 · `degraded` 60–84 · `critical` <60 · `unknown`

Each pipeline also gets **diagnostic hints** — plain-English explanations of what's wrong and how to fix it (queue backpressure, receiver refusals, export failures, retry storms, filter drops, and more).

---

## Supported pipeline components

| Type | Source | What is monitored |
|------|--------|-------------------|
| `otelcol` | OTel Collector `/metrics` | Receiver accepted/refused, exporter sent/failed, processor drops, queue depth |
| `prometheus` | Prometheus `/metrics` | Remote write queue, WAL errors, shard saturation, scrape success |
| `loki` | Loki `/metrics` | Distributor lines received, ingester flush errors, ring health |
| `fluentbit` | Fluent Bit `/api/v1/metrics` | Input records, output sent/errors/retries/retried_failed, filter drops |

**Auth modes:** `mtls` · `apikey` · `bearer` · `basic` · `none`

---

## Architecture

```
Pipeline components (OTel Collector, Prometheus, Loki, Fluent Bit, ...)
         │  HTTP scrape (Prometheus text / JSON)
         ▼
  obsidianstack-agent
  ├── Scrapers       (per source type — otelcol, prometheus, loki, fluentbit)
  ├── Compute Engine (drop%, latency, strength score, per-minute rates)
  └── gRPC Shipper   (mTLS / API key, ring buffer + exponential backoff)
         │  gRPC (protobuf)
         ▼
  obsidianstack-server
  ├── gRPC Receiver  (validates auth, stores snapshots with TTL)
  ├── Diagnostics    (per-source-type hints with actionable detail)
  ├── REST API       (/api/v1/health, /pipelines, /signals, /alerts, ...)
  └── WebSocket      (/ws/stream — live push every 5 s)
         │  HTTP / WebSocket
         ▼
  obsidianstack-ui   (React + Vite + Tailwind)
  ├── Pipeline cards with strength score + health state
  ├── OTel flow card (Receivers → Processors → Exporters per signal type)
  ├── Queue depth bar + backpressure warnings
  └── Diagnostic hints panel
```

---

## Repository layout

```
obsidianstack/
├── agent/                   # Go agent binary
│   ├── cmd/agent/
│   └── internal/
│       ├── config/          # YAML config loader + hot-reload
│       ├── scraper/         # otelcol, prometheus, loki, fluentbit scrapers
│       ├── compute/         # strength score + per-minute delta engine
│       └── shipper/         # gRPC client with ring buffer + retry
├── server/                  # Go server binary
│   ├── cmd/server/
│   └── internal/
│       ├── config/          # server config loader
│       ├── receiver/        # gRPC SnapshotService handler
│       ├── store/           # in-memory snapshot store with TTL
│       ├── auth/            # API key + mTLS interceptors
│       ├── api/             # REST handlers + diagnostics engine
│       ├── ws/              # WebSocket push hub
│       └── alerts/          # rule engine + Slack/Teams webhooks
├── ui/                      # React dashboard
│   └── src/
│       ├── components/      # OtelFlowCard, SignalChip, ...
│       └── pages/           # Pipelines, Health, Signals, Alerts
├── proto/obsidian/v1/       # Protobuf schema (PipelineSnapshot)
├── gen/obsidian/v1/         # Generated gRPC Go code
├── charts/obsidianstack/    # Helm chart
├── deploy/
│   ├── cluster/             # Kubernetes values + OTel Collector manifest
│   └── vm/                  # systemd units + install script
└── Makefile
```

---

## Quickstart

### Kubernetes (Helm)

```bash
helm upgrade --install obsidianstack charts/obsidianstack \
  --namespace obsidianstack \
  --create-namespace \
  --values deploy/cluster/values.yaml
```

`--create-namespace` is all you need — the chart does not try to manage the namespace itself, so there is no chicken-and-egg problem.

Check status:

```bash
kubectl get pods -n obsidianstack
kubectl port-forward -n obsidianstack svc/obsidianstack-ui 8080:80
# open http://localhost:8080
```

### VM / bare metal

The install script pulls pre-built binaries from Docker Hub (requires Docker, no Go needed):

```bash
# Clone the repo, then:
sudo bash deploy/vm/install.sh both          # agent + server

# Edit configs
sudo vi /etc/obsidianstack/agent.yaml        # set sources + server_endpoint
sudo vi /etc/obsidianstack/agent.env         # set PROM_PASSWORD etc.

sudo systemctl start obsidianstack-agent obsidianstack-server
sudo journalctl -fu obsidianstack-agent
```

To build from source instead (requires Go 1.24+):

```bash
sudo bash deploy/vm/install.sh both --from-source
```

### Docker images

Pre-built multi-arch (amd64 + arm64) images on Docker Hub:

```
marocz/obsidianstack-agent:latest
marocz/obsidianstack-server:latest
marocz/obsidianstack-ui:latest
```

---

## Configuration

### Agent

```yaml
agent:
  server_endpoint: "obsidianstack-server:50051"
  scrape_interval: 15s
  ship_interval:   15s
  buffer_size:     1000

  sources:
    # OTel Collector
    - id: "otel-collector"
      type: otelcol
      endpoint: "http://otelcol.monitoring:8888/metrics"

    # Prometheus
    - id: "prometheus"
      type: prometheus
      endpoint: "http://prometheus.monitoring:9090/metrics"

    # Prometheus with basic auth
    - id: "prometheus-prod"
      type: prometheus
      endpoint: "https://prom.example.com/metrics"
      auth:
        mode: basic
        username: "admin"
        password_env: PROM_PASSWORD   # set in agent.env

    # Loki
    - id: "loki"
      type: loki
      endpoint: "http://loki.logging:3100/metrics"

    # Fluent Bit
    - id: "fluent-bit"
      type: fluentbit
      endpoint: "http://fluent-bit.logging:2020"

    # mTLS example
    - id: "secure-otel"
      type: otelcol
      endpoint: "https://otelcol.internal:8888/metrics"
      auth:
        mode: mtls
        cert_file: /etc/certs/client.crt
        key_file:  /etc/certs/client.key
        ca_file:   /etc/certs/ca.crt
```

### Server

```yaml
server:
  grpc_port: 50051
  http_port:  8080
  auth:
    mode: none   # set to apikey for production
  alerts:
    rules:
      - name: "high-drop-rate"
        condition: "drop_pct > 5"
        severity: critical
        cooldown: 15m
      - name: "pipeline-critical"
        condition: "state == critical"
        severity: critical
        cooldown: 5m
      - name: "cert-expiring"
        condition: "cert_days_left < 30"
        severity: warning
        cooldown: 24h
    webhooks:
      - type: slack
        url_env: SLACK_WEBHOOK_URL
```

---

## REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Overall health score, state, pipeline counts |
| GET | `/api/v1/pipelines` | All pipelines with score, diagnostics, extra metrics |
| GET | `/api/v1/pipelines/{id}` | Single pipeline detail |
| GET | `/api/v1/signals` | Aggregated metrics / logs / traces breakdown |
| GET | `/api/v1/alerts` | Active alert list |
| GET | `/api/v1/certs` | TLS certificate status per source |
| GET | `/api/v1/snapshot` | Full JSON dump of all pipeline state |
| WS  | `/ws/stream` | Live push stream (JSON, every 5 s) |

---

## Development status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Agent core — scraper, compute engine, gRPC shipper | ✅ Complete |
| 2 | Server — gRPC receiver, REST API, in-memory store | ✅ Complete |
| 3 | Security — mTLS, API key, bearer auth | ✅ Complete |
| 4 | React UI dashboard + WebSocket live push | ✅ Complete |
| 5 | Helm chart + Kubernetes manifests | ✅ Complete |
| 6 | VM deploy — systemd units + install script | ✅ Complete |
| 7 | Alerting — rule engine + Slack/Teams webhooks | ✅ Complete |
| 7b | OTel Collector flow visualisation (queue, signals, extra metrics) | ✅ Complete |
| 7c | Fluent Bit scraper support | ✅ Complete |
| 8 | Historical storage + trend charts | 🔲 Planned |

---

## Contributing / Release workflow

All changes go through feature branches — direct pushes to `main` are blocked.

### Making a change

```bash
git checkout -b feature/my-change
# ... make changes ...
make test                              # all tests must pass
git push origin feature/my-change
gh pr create                           # open PR, get review, merge
```

### Building and testing images locally

Before pushing images to Docker Hub, build and smoke-test locally:

```bash
# Build all three images (single-arch, your machine's native arch)
make docker-build VERSION=v0.1.1 DOCKER_USER=marocz

# Quick smoke test with docker-compose
docker compose up

# Or test against your Kubernetes cluster:
#   Update deploy/cluster/values.yaml image tags to v0.1.1, then:
make helm-upgrade-cluster
make rollout-restart
kubectl get pods -n obsidianstack
```

### Pushing images to Docker Hub

Once you've confirmed the images work:

```bash
# Login once if needed
docker login

# Push multi-arch images (amd64 + arm64) to Docker Hub
make docker-push VERSION=v0.1.1 DOCKER_USER=marocz
```

### Releasing a new version

After the PR is merged to `main`, tag the release. GitHub Actions builds
the official multi-arch images and creates a GitHub Release automatically:

```bash
git checkout main && git pull
git tag v0.1.1
git push origin v0.1.1
```

The CI workflow will publish:
- `marocz/obsidianstack-agent:v0.1.1`
- `marocz/obsidianstack-server:v0.1.1`
- `marocz/obsidianstack-ui:v0.1.1`

Then update the cluster to the new tag:

```bash
# Edit deploy/cluster/values.yaml — set all three image tags to v0.1.1
helm upgrade --install obsidianstack charts/obsidianstack \
  --namespace obsidianstack \
  --create-namespace \
  --values deploy/cluster/values.yaml
```

### Useful Makefile targets

```
make test                    # Run all Go tests with race detector
make docker-build            # Build all images locally (single-arch)
make docker-push             # Push multi-arch images to Docker Hub
make release                 # Run tests + build + print release checklist
make helm-upgrade-cluster    # Deploy to k8s using deploy/cluster/values.yaml
make rollout-restart         # Restart all pods to pick up new images
make helm-template-cluster   # Dry-run render with cluster values
```

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
