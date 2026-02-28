package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// OTel Collector base metric names. Each comes in three signal-type suffixes:
// _spans (traces), _metric_points (metrics), _log_records (logs).
const (
	otelReceiverAccepted = "otelcol_receiver_accepted"
	otelReceiverRefused  = "otelcol_receiver_refused"
	otelExporterSent     = "otelcol_exporter_sent"
	otelExporterFailed   = "otelcol_exporter_send_failed"
	otelProcessorDropped = "otelcol_processor_dropped"
)

// otelSuffixes maps the OTel metric suffix to the canonical signal type.
var otelSuffixes = map[string]string{
	"spans":         "traces",
	"metric_points": "metrics",
	"log_records":   "logs",
}

type otelScraper struct {
	src    config.Source
	client *http.Client
}

// Scrape fetches the OTel Collector's internal Prometheus metrics endpoint and
// returns received/dropped counts per signal type (metrics, logs, traces).
//
// Dropped items include exporter send failures and processor drops.
// Receiver refusals are tracked in Extra["receiver_refused_*"] for diagnostics
// but excluded from the drop count (they never entered the pipeline).
func (s *otelScraper) Scrape(ctx context.Context) (*ScrapeResult, error) {
	res := newResult(s.src.ID, "otelcol")

	mfs, err := fetchMetrics(ctx, s.client, s.src.Endpoint)
	if err != nil {
		res.Err = fmt.Errorf("otelcol scrape %q: %w", s.src.ID, err)
		slog.Warn("scraper: otelcol fetch failed", "source", s.src.ID, "err", err)
		return res, nil // return partial result; Err signals health Unknown
	}

	for suffix, signal := range otelSuffixes {
		accepted := sumFamily(mfs[otelReceiverAccepted+"_"+suffix+"_total"])
		refused := sumFamily(mfs[otelReceiverRefused+"_"+suffix+"_total"])
		sent := sumFamily(mfs[otelExporterSent+"_"+suffix+"_total"])
		failed := sumFamily(mfs[otelExporterFailed+"_"+suffix+"_total"])
		procDropped := sumFamily(mfs[otelProcessorDropped+"_"+suffix+"_total"])

		res.Received[signal] += accepted

		// Dropped = exporter failures + processor drops.
		// We use this rather than (accepted - sent) to avoid negative values
		// caused by counter resets after restarts.
		res.Dropped[signal] += failed + procDropped

		// Detailed breakdown stored in Extra for the compute engine.
		res.Extra["receiver_accepted_"+suffix] = accepted
		res.Extra["receiver_refused_"+suffix] = refused
		res.Extra["exporter_sent_"+suffix] = sent
		res.Extra["exporter_send_failed_"+suffix] = failed
		res.Extra["processor_dropped_"+suffix] = procDropped
	}

	// Queue depth metrics — useful for detecting backpressure before drops occur.
	res.Extra["exporter_queue_size"] = sumFamily(mfs["otelcol_exporter_queue_size"])
	res.Extra["exporter_queue_capacity"] = sumFamily(mfs["otelcol_exporter_queue_capacity"])

	return res, nil
}
