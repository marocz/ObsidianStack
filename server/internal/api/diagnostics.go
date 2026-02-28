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

// otelcolHints generates OTel-Collector-specific diagnostic hints using the
// Extra map (queue gauges + per-minute counter rates populated by the agent).
func otelcolHints(snap *pb.PipelineSnapshot) []DiagnosticHint {
	ex := snap.Extra // may be nil for first scrape
	var hints []DiagnosticHint

	// ── Queue backpressure ────────────────────────────────────────────────────
	qSize := ex["exporter_queue_size"]
	qCap := ex["exporter_queue_capacity"]
	if qCap > 0 {
		fillPct := qSize / qCap * 100
		v := fillPct
		switch {
		case fillPct >= 90:
			hints = append(hints, DiagnosticHint{
				Key:   "otel_queue_critical",
				Level: "critical",
				Title: fmt.Sprintf("Queue %.0f%% full", fillPct),
				Detail: fmt.Sprintf(
					"The OTel Collector exporter queue is %.0f%% full (%.0f / %.0f slots). "+
						"This means your downstream backends (Prometheus remote write, Loki) "+
						"cannot keep up with the ingest rate. Data will start dropping imminently. "+
						"Immediate actions: scale up the backend, increase queue_size in your "+
						"exporter config (sending_queue.queue_size), or add more exporter workers "+
						"(sending_queue.num_consumers). Check otelcol_exporter_send_failed_* for failures.",
					fillPct, qSize, qCap,
				),
				Value: &v,
			})
		case fillPct >= 70:
			hints = append(hints, DiagnosticHint{
				Key:   "otel_queue_warning",
				Level: "warning",
				Title: fmt.Sprintf("Queue %.0f%% full", fillPct),
				Detail: fmt.Sprintf(
					"The OTel Collector exporter queue is %.0f%% full (%.0f / %.0f slots). "+
						"Backpressure is building — if ingest continues at this rate without "+
						"the backend catching up, data will start dropping. "+
						"Consider scaling your backend or increasing the queue size before it reaches 90%%.",
					fillPct, qSize, qCap,
				),
				Value: &v,
			})
		case fillPct >= 30:
			hints = append(hints, DiagnosticHint{
				Key:    "otel_queue_ok",
				Level:  "info",
				Title:  fmt.Sprintf("Queue %.0f%% used", fillPct),
				Detail: fmt.Sprintf("The exporter queue is %.0f%% full (%.0f / %.0f). Healthy headroom.", fillPct, qSize, qCap),
				Value:  &v,
			})
		}
	}

	// ── Receiver refusals (items rejected before entering the pipeline) ───────
	var totalRefusedPM float64
	for _, suffix := range []string{"spans", "metric_points", "log_records"} {
		totalRefusedPM += ex["receiver_refused_"+suffix+"_pm"]
	}
	if totalRefusedPM > 0.5 {
		v := totalRefusedPM
		hints = append(hints, DiagnosticHint{
			Key:   "otel_receiver_refused",
			Level: "warning",
			Title: fmt.Sprintf("%.0f items/min refused", totalRefusedPM),
			Detail: fmt.Sprintf(
				"The OTel Collector is refusing %.0f items per minute at the receiver stage — "+
					"these are items that never even entered the pipeline. "+
					"This usually means the collector is overwhelmed or a memory_limiter processor "+
					"is rejecting data to protect itself. "+
					"Check otelcol_receiver_refused_* metrics and consider increasing memory limits "+
					"or reducing the upstream send rate.",
				totalRefusedPM,
			),
			Value: &v,
		})
	}

	// ── Export failures ───────────────────────────────────────────────────────
	var totalFailedPM float64
	for _, suffix := range []string{"spans", "metric_points", "log_records"} {
		totalFailedPM += ex["exporter_send_failed_"+suffix+"_pm"]
	}
	if totalFailedPM > 0.5 {
		v := totalFailedPM
		hints = append(hints, DiagnosticHint{
			Key:   "otel_export_failures",
			Level: "critical",
			Title: fmt.Sprintf("%.0f exports/min failing", totalFailedPM),
			Detail: fmt.Sprintf(
				"%.0f items per minute are failing to export to their destinations. "+
					"This is distinct from queue pressure — these are items the collector "+
					"tried to send but the backend rejected or dropped the connection. "+
					"Check the logs for exporter errors: `kubectl logs -n monitoring deploy/otel-collector`. "+
					"Common causes: backend authentication failure, TLS errors, backend overload, "+
					"or network connectivity issues between the collector and your storage backends.",
				totalFailedPM,
			),
			Value: &v,
		})
	}

	// ── Uptime context for otelcol ────────────────────────────────────────────
	if snap.UptimePct < 100 {
		hints = append(hints, DiagnosticHint{
			Key:   "otel_restart_tip",
			Level: "info",
			Title: "Check collector logs",
			Detail: "When the OTel Collector restarts it loses any data buffered in memory " +
				"(unless you have persistent queue enabled). " +
				"Run `kubectl logs -n monitoring deploy/otel-collector --previous` to see why it restarted. " +
				"Common causes: OOM (increase memory limit or tighten memory_limiter), " +
				"config errors, or upstream connectivity issues.",
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
		hints = append(hints, otelcolHints(snap)...)

	case "fluentbit":
		hints = append(hints, fluentbitHints(snap)...)
	}

	return hints
}

// fluentbitHints generates Fluent Bit-specific diagnostic hints using the
// Extra map (per-minute counter rates populated by the compute engine).
func fluentbitHints(snap *pb.PipelineSnapshot) []DiagnosticHint {
	ex := snap.Extra
	var hints []DiagnosticHint

	// ── Permanent data loss (retried_failed = max retries exhausted) ──────────
	lostPM := ex["output_retried_failed_pm"]
	if lostPM > 0 {
		v := lostPM
		hints = append(hints, DiagnosticHint{
			Key:   "fb_data_loss",
			Level: "critical",
			Title: fmt.Sprintf("%.0f records/min lost", lostPM),
			Detail: fmt.Sprintf(
				"Fluent Bit is permanently losing %.0f log records per minute. "+
					"These are records that failed to reach an output plugin and exhausted "+
					"all retry attempts — they are gone. "+
					"Check your output plugin configuration: is the destination reachable? "+
					"Is the endpoint returning errors? You can also check `output_errors_pm` "+
					"and `output_retries_pm` to understand the failure pattern. "+
					"Consider enabling the filesystem buffer (`storage.type filesystem`) "+
					"so records survive Fluent Bit restarts.",
				lostPM,
			),
			Value: &v,
		})
	}

	// ── Output errors (failed sends, before retries exhaust) ──────────────────
	errorsPM := ex["output_errors_pm"]
	if errorsPM > 0.5 {
		v := errorsPM
		hints = append(hints, DiagnosticHint{
			Key:   "fb_output_errors",
			Level: "warning",
			Title: fmt.Sprintf("%.0f output errors/min", errorsPM),
			Detail: fmt.Sprintf(
				"Fluent Bit is encountering %.0f output errors per minute. "+
					"Errors trigger retries — if retries keep failing they become permanent loss. "+
					"Common causes: destination unreachable, authentication failure, "+
					"TLS certificate issues, or the backend is rate-limiting. "+
					"Check Fluent Bit logs: `kubectl logs <fluent-bit-pod>` or `journalctl -u td-agent-bit`.",
				errorsPM,
			),
			Value: &v,
		})
	}

	// ── Retry storm (backpressure building up) ────────────────────────────────
	retriesPM := ex["output_retries_pm"]
	if retriesPM > 5 && lostPM == 0 {
		// Only show if no data loss yet — if there IS loss, the critical hint covers it.
		v := retriesPM
		hints = append(hints, DiagnosticHint{
			Key:   "fb_retries",
			Level: "info",
			Title: fmt.Sprintf("%.0f retries/min", retriesPM),
			Detail: fmt.Sprintf(
				"Fluent Bit is retrying %.0f times per minute. No data is lost yet, "+
					"but sustained retries indicate your output destination is struggling. "+
					"If retries keep increasing, records will eventually exhaust the retry limit "+
					"and be permanently dropped. Monitor `output_retried_failed_pm` closely.",
				retriesPM,
			),
			Value: &v,
		})
	}

	// ── Filter drops ──────────────────────────────────────────────────────────
	filterDropPM := ex["filter_drop_records_pm"]
	if filterDropPM > 0 {
		v := filterDropPM
		hints = append(hints, DiagnosticHint{
			Key:   "fb_filter_drops",
			Level: "info",
			Title: fmt.Sprintf("%.0f records/min filtered", filterDropPM),
			Detail: fmt.Sprintf(
				"%.0f log records per minute are being intentionally dropped by filter plugins "+
					"(grep, lua, etc.). This is normal if you have filtering rules configured. "+
					"If this number is higher than expected, check your filter configurations "+
					"to make sure you're not accidentally dropping logs you need.",
				filterDropPM,
			),
			Value: &v,
		})
	}

	return hints
}
