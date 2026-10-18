# Roadmap

Phases 1–7d are done (see the status table in the [README](README.md)). This is the work ahead, roughly in order.
Each item has a matching GitHub issue.

## Phase 8 — History and trends

The server keeps only the latest snapshot per pipeline in memory, so a restart loses everything and there is no way to
see how a pipeline behaved an hour ago.

- [ ] **Persistent snapshot store** — pluggable `store` interface with a SQLite backend (default) next to the current
      in-memory one; configurable retention and downsampling (raw 24h, 1-minute rollups 7d).
- [ ] **History API** — `GET /api/v1/pipelines/{id}/history?from=&to=&step=` returning score, drop %, latency and
      throughput series.
- [ ] **Trend charts in the UI** — sparklines on the Pipelines page and a full chart on pipeline detail, with a time
      range picker.
- [ ] **Helm/VM support for persistence** — PVC in the chart, data dir in the systemd unit, documented backups.

## Phase 9 — Better alerting

- [ ] **Alert history and acknowledgement** — store fired/resolved alerts, show them in the UI, allow ack/silence.
- [ ] **More notification channels** — generic webhook, email (SMTP) and PagerDuty Events v2, alongside Slack/Teams.
- [ ] **Trend-based rules** — alert on "drop rate rising for 10 minutes", not only on thresholds.

## Phase 10 — More sources and integrations

- [ ] **Vector scraper** — `vector` source type using Vector's internal metrics.
- [ ] **Grafana Tempo / Mimir scrapers** — distributor and ingester health for traces and long-term metrics.
- [ ] **Expose ObsidianStack's own metrics** — `/metrics` on the server with pipeline scores, so the scores can be graphed
      in existing Grafana setups.
- [ ] **Add sources without editing YAML** — "Add source" currently generates YAML to paste into the agent config; add
      a server-side source registry that agents pull and hot-reload.

## Quality and docs

- [ ] **UI tests** — Vitest + React Testing Library for hooks and the main pages; run in CI.
- [ ] **End-to-end test** — docker compose stack (OTel Collector + Prometheus + Loki + agent + server) driven from CI,
      asserting scores move when a fault is injected.
- [ ] **Publish the Helm chart** — chart repository on GitHub Pages, released from CI on tags.
- [ ] **Docs** — scoring model explained in depth, troubleshooting guide, and a "first 10 minutes" tutorial.
