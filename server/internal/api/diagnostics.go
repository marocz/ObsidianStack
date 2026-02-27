package api

import (
	"fmt"
	"strings"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
)

// DiagnosticHint is one human-readable insight about a pipeline's health.
// The UI displays these as chips on the pipeline card; clicking one shows
// Detail — written like an AI assistant explaining the problem in plain English.
type DiagnosticHint struct {
	// Key is a stable machine-readable identifier (used for dedup/ordering).
	Key string `json:"key"`
	// Level is "ok" | "info" | "warning" | "critical"
	Level string `json:"level"`
	// Title is a short label shown on the chip (≤ 5 words).
	Title string `json:"title"`
	// Detail is the full explanation shown on click/hover.
	Detail string `json:"detail"`
	// Value is an optional numeric value associated with this hint (e.g. drop %).
	Value *float64 `json:"value,omitempty"`
}

// computeDiagnostics derives human-readable diagnostic hints from a snapshot.
// Diagnostics are ordered: critical first, then warnings, then info.
func computeDiagnostics(snap *pb.PipelineSnapshot) []DiagnosticHint {
	var hints []DiagnosticHint

	// ── Scrape failure ───────────────────────────────────────────────────────
	if snap.ErrorMessage != "" {
		msg := snap.ErrorMessage
		detail := fmt.Sprintf(
			"The agent couldn't collect data from this source. "+
				"It last tried and got: \"%s\". "+
				"Check that the endpoint is reachable, your credentials are correct, "+
				"and the service is running. Until this is resolved, all health metrics "+
				"for this pipeline are unavailable.",
			msg,
		)
		hints = append(hints, DiagnosticHint{
			Key:    "scrape_failed",
			Level:  "critical",
			Title:  "Can't reach source",
			Detail: detail,
		})
		return hints // no point computing further without data
	}

	// ── Unknown state (first scrape / no baseline yet) ───────────────────────
	if snap.State == "unknown" && snap.DropPct == 0 && snap.ThroughputPerMin == 0 {
		hints = append(hints, DiagnosticHint{
			Key:   "warming_up",
			Level: "info",
			Title: "Warming up",
			Detail: "The agent is collecting its first data point. " +
				"Health metrics are calculated from the difference between two consecutive scrapes, " +
				"so everything will show up after the next scrape cycle (about 15 seconds). " +
				"No action needed.",
		})
		return hints
	}

	// ── Data loss ─────────────────────────────────────────────────────────────
	if snap.DropPct > 0 {
		pct := snap.DropPct
		v := pct
		var level, title, detail string

		perMin := snap.ThroughputPerMin * (pct / 100)

		switch {
		case pct >= 10:
			level = "critical"
			title = fmt.Sprintf("%.1f%% data loss", pct)
			detail = fmt.Sprintf(
				"This pipeline is losing %.1f%% of its data — roughly %.0f items per minute "+
					"are being dropped. At this rate you are missing significant chunks of your "+
					"observability signal. Common causes: your remote storage is overwhelmed, "+
					"the write queue is full, or a downstream exporter is failing. "+
					"Check your remote write targets and backend storage capacity.",
				pct, perMin,
			)
		case pct >= 1:
			level = "warning"
			title = fmt.Sprintf("%.1f%% drop rate", pct)
			detail = fmt.Sprintf(
				"About %.1f%% of data is being dropped (≈ %.0f items/min). "+
					"This is worth investigating — it often means a downstream system "+
					"is under pressure or the pipeline queue is filling up. "+
					"Monitor whether this number is growing.",
				pct, perMin,
			)
		default:
			level = "info"
			title = fmt.Sprintf("%.2f%% minor drops", pct)
			detail = fmt.Sprintf(
				"A very small amount of data (%.2f%%) is being dropped. "+
					"This may be normal jitter, but keep an eye on it in case it grows.",
				pct,
			)
		}
		hints = append(hints, DiagnosticHint{Key: "drop_rate", Level: level, Title: title, Detail: detail, Value: &v})
	}

	// ── Recovery rate (when there are drops) ─────────────────────────────────
	if snap.DropPct > 0 && snap.RecoveryRate > 0 && snap.RecoveryRate < 100 {
		v := snap.RecoveryRate
		detail := fmt.Sprintf(
			"Of the data that was at risk, %.0f%% is getting through successfully. "+
				"Recovery rate measures the proportion of your pipeline that is healthy. "+
				"A rate below 80%% means a significant portion of data is permanently lost.",
			snap.RecoveryRate,
		)
		hints = append(hints, DiagnosticHint{
			Key:    "recovery_rate",
			Level:  "info",
			Title:  fmt.Sprintf("%.0f%% recovery", snap.RecoveryRate),
			Detail: detail,
			Value:  &v,
		})
	}

	// ── Uptime / restarts ─────────────────────────────────────────────────────
	if snap.UptimePct < 100 && snap.UptimePct > 0 {
		v := snap.UptimePct
		var level string
		switch {
		case snap.UptimePct < 70:
			level = "critical"
		case snap.UptimePct < 90:
			level = "warning"
		default:
			level = "info"
		}
		detail := fmt.Sprintf(
			"This pipeline has been reachable for %.0f%% of recent scrape attempts "+
				"(we sample every 15 seconds, tracking the last 20 results). "+
				"Anything below 100%% means the agent couldn't reach it at least once. "+
				"Look for pod restarts, OOMKilled events, or network issues. "+
				"A brief dip is often a rolling restart; a sustained dip indicates instability.",
			snap.UptimePct,
		)
		hints = append(hints, DiagnosticHint{
			Key:    "uptime",
			Level:  level,
			Title:  fmt.Sprintf("%.0f%% uptime", snap.UptimePct),
			Detail: detail,
			Value:  &v,
		})
	}

	// ── Signal-level breakdown ────────────────────────────────────────────────
	for _, sig := range snap.Signals {
		if sig.DropPct < 0.01 {
			continue
		}
		var sigName string
		switch sig.Type {
		case "metrics":
			sigName = "metric samples"
		case "logs":
			sigName = "log lines"
		case "traces":
			sigName = "trace spans"
		default:
			sigName = sig.Type
		}
		v := sig.DropPct
		detail := fmt.Sprintf(
			"Specifically your %s are seeing a %.1f%% drop rate "+
				"(%.0f dropped per min out of %.0f received). "+
				"This is useful for pinpointing which signal type is under pressure — "+
				"for example, metrics could be healthy while logs are backed up.",
			sigName, sig.DropPct, sig.DroppedPm, sig.ReceivedPm,
		)
		hints = append(hints, DiagnosticHint{
			Key:    "signal_drop_" + sig.Type,
			Level:  "warning",
			Title:  fmt.Sprintf("%s drops", strings.Title(sig.Type)), //nolint:staticcheck
			Detail: detail,
			Value:  &v,
		})
	}

	// ── Source-type specific guidance ─────────────────────────────────────────
	hints = append(hints, sourceTypeHints(snap)...)

	// ── All clear ─────────────────────────────────────────────────────────────
	if len(hints) == 0 {
		score := snap.StrengthScore
		hints = append(hints, DiagnosticHint{
			Key:   "healthy",
			Level: "ok",
			Title: "All clear",
			Detail: fmt.Sprintf(
				"This pipeline is fully operational with a health score of %.0f/100. "+
					"No drops, no scrape errors, and uptime is solid. "+
					"Keep an eye on the throughput trend — a sudden drop in volume "+
					"can indicate an upstream problem even when drop rate is zero.",
				score,
			),
			Value: &score,
		})
	}

	return hints
}

// sourceTypeHints returns source-type-specific diagnostic hints.
func sourceTypeHints(snap *pb.PipelineSnapshot) []DiagnosticHint {
	var hints []DiagnosticHint

	switch snap.SourceType {
	case "prometheus":
		if snap.DropPct > 0 {
			hints = append(hints, DiagnosticHint{
				Key:   "prom_remotewrite_tip",
				Level: "info",
				Title: "Remote write check",
				Detail: "For Prometheus drop issues, start with the remote write queue: " +
					"check prometheus_remote_storage_samples_pending vs queue_capacity. " +
					"If the queue is >80% full, your remote storage backend is too slow. " +
					"You can also check prometheus_tsdb_wal_storage_errors_total for " +
					"local WAL issues, and prometheus_remote_storage_shards for shard saturation.",
			})
		}
		if snap.UptimePct < 100 {
			hints = append(hints, DiagnosticHint{
				Key:   "prom_restart_tip",
				Level: "info",
				Title: "Check Prometheus logs",
				Detail: "When Prometheus restarts, it replays the WAL before accepting scrapes again. " +
					"This can cause a brief gap in your metrics. " +
					"Run `kubectl logs -n monitoring <prometheus-pod> --previous` to see why it restarted. " +
					"Common causes: OOM (increase memory limit), storage full (increase PVC size), " +
					"or liveness probe too aggressive.",
			})
		}

	case "loki":
		if snap.DropPct > 0 {
			hints = append(hints, DiagnosticHint{
				Key:   "loki_flush_tip",
				Level: "info",
				Title: "Check Loki ingesters",
				Detail: "Loki drops usually happen when the ingester can't flush chunks to storage. " +
					"Check loki_ingester_flush_failures_total for flush errors, " +
					"and loki_ingester_ingestion_rate_bytes to see if you're near your ingestion limit. " +
					"Common fixes: increase the ingestion rate limit in Loki config, " +
					"scale the backend storage, or add more ingester replicas.",
			})
		}

	case "otelcol":
		if snap.DropPct > 0 {
			hints = append(hints, DiagnosticHint{
				Key:   "otel_exporter_tip",
				Level: "info",
				Title: "Check OTel exporters",
				Detail: "OpenTelemetry Collector drops happen at two points: " +
					"(1) the processor stage (check otelcol_processor_dropped_*) and " +
					"(2) the exporter stage (check otelcol_exporter_send_failed_*). " +
					"Also look at otelcol_exporter_queue_size vs queue_capacity — " +
					"if the queue is filling up, your backend can't keep up with the ingest rate. " +
					"Consider increasing the queue size or adding retry with backoff in your exporter config.",
			})
		}
	}

	return hints
}
