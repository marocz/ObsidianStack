# How ObsidianStack works and how to run it locally

## The three pieces

```
┌─────────────────────────────────────────────────────────────┐
│  Browser  →  UI (React, localhost:3000)                     │
│               │  REST/WebSocket                             │
│               ↓                                             │
│  Server   →  obsidianstack-server  (localhost:8080 / :50051)│
│               │  gRPC                                       │
│               ↓                                             │
│  Agent    →  obsidianstack-agent                            │
│               │  HTTP scrape                                │
│               ↓                                             │
│  Sources  →  Prometheus, Loki, OTel Collector …             │
└─────────────────────────────────────────────────────────────┘
```

### obsidianstack-agent
- **What it does**: scrapes your Prometheus/Loki/OTel endpoints, computes a
  health score, and ships the result to the server over gRPC every 15 seconds.
- **It does NOT serve any UI or HTTP API** — it only dials outward.
- **Needs to run wherever it can reach your sources** (on the same cluster node,
  or your laptop with port-forwards for local dev).

### obsidianstack-server
- **What it does**: receives snapshots from agents over gRPC (:50051), stores
  them in memory, evaluates alert rules, and exposes a REST + WebSocket API
  on :8080.
- **Does NOT scrape anything** — it only receives data from agents.

### UI (React / Vite)
- **What it does**: talks to the server's REST API and WebSocket to display
  pipeline health, signals, alerts and certs.
- **In dev**: runs as a Vite dev server on :3000 (proxied to :8080).
- **In production**: built to static files and served by nginx or the server
  binary (future).
- The UI is **not** part of the agent or server binary — it is a separate
  process during development.

---

## Local development — step by step

### Prerequisites
- Go 1.24 (`/opt/homebrew/opt/go@1.24/bin/go`)
- Node 18+ + npm
- kubectl configured to your cluster (for port-forwards)

### 1. Build the binaries (one time, or after any Go change)

```bash
make build
# Produces: bin/obsidianstack-agent  bin/obsidianstack-server
```

### 2. Port-forward cluster sources (keep these running in background terminals)

```bash
# Terminal A — Prometheus
kubectl port-forward -n monitoring \
  pod/$(kubectl get pod -n monitoring -l app.kubernetes.io/name=prometheus \
  -o jsonpath='{.items[0].metadata.name}') 9090:9090

# Terminal B — Loki
kubectl port-forward -n loki svc/loki 3100:3100
```

### 3. Set secrets for external sources

```bash
# Only needed for sources that use basic/bearer/apikey auth.
# The value must be the PLAIN-TEXT password, not an htpasswd hash.
export PROM_PASSWORD='the-real-plaintext-password'
```

### 4. Start the server (Terminal C)

```bash
./bin/obsidianstack-server -config config/server.yaml
```

Logs appear in the terminal. Server is ready when you see:
```
{"msg":"gRPC receiver listening","port":50051}
{"msg":"HTTP server listening","port":8080}
```

### 5. Start the agent (Terminal D — same shell where you exported PROM_PASSWORD)

```bash
./bin/obsidianstack-agent -config config/agent.yaml
```

After ~15 seconds (first scrape) you will see:
```
{"msg":"shipper: connected","endpoint":"localhost:50051"}
```

### 6. Start the UI (Terminal E)

```bash
make run-ui
# or:  cd ui && npm run dev
```

Open http://localhost:3000 — you will see all configured sources.

---

## Where each config lives

| File | What to edit |
|---|---|
| `config/agent.yaml` | Sources (endpoints, auth, IDs) |
| `config/server.yaml` | Alert rules, webhook URLs |
| Shell `export` or `.env` | Passwords / tokens (never put in YAML) |

---

## Why `prometheus-prod` shows as `unknown`

The source at `prom-staging.vechtron.com` uses HTTP Basic Auth.
The `$apr1$...` string you see is an **Apache htpasswd hash** — that is what
the server stores. The agent needs the original **plain-text** password:

```bash
export PROM_PASSWORD='the-original-password-before-htpasswd-hashed-it'
# Then restart the agent — it will pick up the env var immediately.
```

If you don't know the plain-text password, reset it on the server:
```bash
htpasswd /etc/nginx/.htpasswd admin   # type a new password at the prompt
# Then export PROM_PASSWORD='that-new-password' on your laptop
```

---

## Deploying to Kubernetes with Helm

### Quick install (default values)

```bash
helm upgrade --install obsidianstack charts/obsidianstack \
  --namespace obsidianstack \
  --create-namespace
```

### With your sources and secrets

Create a `my-values.yaml`:
```yaml
agent:
  image:
    repository: YOURDOCKERHUB/obsidianstack-agent
    tag: v0.1.0
  config: |
    server_endpoint: "obsidianstack-server:50051"
    scrape_interval: 15s
    sources:
      - id: "prometheus"
        type: prometheus
        endpoint: "http://prometheus.monitoring:9090/metrics"
      - id: "loki"
        type: loki
        endpoint: "http://loki.loki:3100/metrics"
      - id: "otel-collector"
        type: otelcol
        endpoint: "http://otel-collector.monitoring:8888/metrics"
  secrets:
    PROM_PASSWORD: "your-plain-text-password"

server:
  image:
    repository: YOURDOCKERHUB/obsidianstack-server
    tag: v0.1.0
  ingress:
    enabled: true
    className: nginx
    host: obs.yourdomain.com
    tls: true
  secrets:
    SLACK_WEBHOOK_URL: "https://hooks.slack.com/services/..."
```

```bash
helm upgrade --install obsidianstack charts/obsidianstack \
  --namespace obsidianstack \
  --create-namespace \
  --values my-values.yaml
```

### Build and push Docker images

```bash
# One-time: create a buildx builder for multi-arch
docker buildx create --use --name multiarch

# Push to Docker Hub (you must be logged in: docker login)
make docker-push DOCKER_USER=yourname VERSION=v0.1.0
```

Or let GitHub Actions do it automatically: push a tag and the workflow fires:
```bash
git tag v0.1.0 && git push origin v0.1.0
```

Requires two GitHub secrets: `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`
(Settings → Secrets → Actions → New repository secret).

---

## Adding an OTel Collector source

The agent natively understands OTel Collector's internal metrics.

Enable metrics on your OTel Collector (`config.yaml`):
```yaml
service:
  telemetry:
    metrics:
      address: 0.0.0.0:8888
```

Add to `config/agent.yaml`:
```yaml
sources:
  - id: "otel-collector"
    type: otelcol
    endpoint: "http://otel-collector:8888/metrics"
```

The agent will report:
- **Received / dropped** per signal type (spans, metric points, log records)
- **Exporter queue depth** — early backpressure warning before drops occur
- **Receiver refusals** — upstream rejection count

---

## Stopping everything

```bash
make stop                     # kills agent + server
# kill the port-forward terminals manually (Ctrl-C)
# Ctrl-C in the UI terminal
```
