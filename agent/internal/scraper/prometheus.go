package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// Prometheus internal metric names we track.
const (
	// TSDB ingestion counter — total samples written to the local TSDB head.
	promSamplesAppended = "prometheus_tsdb_head_samples_appended_total"

	// Remote write drop counter — samples that could not be sent and were dropped.
	promSamplesDropped = "prometheus_remote_storage_samples_dropped_total"

	// Remote write success counter — samples successfully delivered to a remote.
	promSamplesSent = "prometheus_remote_storage_succeeded_samples_total"

	// Remote write queue depth — current number of samples waiting to be sent.
	promQueuePending = "prometheus_remote_storage_samples_pending"

	// Remote write queue capacity — maximum samples the queue can hold.
	promQueueCapacity = "prometheus_remote_storage_queue_capacity"

	// Shards currently active for remote write.
	promShardsActive = "prometheus_remote_storage_shards"

	// WAL storage errors — unrecoverable write errors to local WAL.
	promWALErrors = "prometheus_tsdb_wal_storage_errors_total"
)

type promScraper struct {
	src    config.Source
	client *http.Client
}

// Scrape fetches Prometheus's own /metrics endpoint and extracts ingestion
// and remote-write health data.
//
// All signal data is reported under the "metrics" signal type since Prometheus
// only handles metric samples.
func (s *promScraper) Scrape(ctx context.Context) (*ScrapeResult, error) {
	res := newResult(s.src.ID, "prometheus")

	mfs, err := fetchMetrics(ctx, s.client, s.src.Endpoint)
	if err != nil {
		res.Err = fmt.Errorf("prometheus scrape %q: %w", s.src.ID, err)
		slog.Warn("scraper: prometheus fetch failed", "source", s.src.ID, "err", err)
		return res, nil
	}

	appended := sumFamily(mfs[promSamplesAppended])
	dropped := sumFamily(mfs[promSamplesDropped])
	sent := sumFamily(mfs[promSamplesSent])

	res.Received["metrics"] = appended
	res.Dropped["metrics"] = dropped

	res.Extra["samples_appended"] = appended
	res.Extra["samples_dropped"] = dropped
	res.Extra["samples_sent"] = sent
	res.Extra["queue_pending"] = sumFamily(mfs[promQueuePending])
	res.Extra["queue_capacity"] = sumFamily(mfs[promQueueCapacity])
	res.Extra["shards_active"] = sumFamily(mfs[promShardsActive])
	res.Extra["wal_errors"] = sumFamily(mfs[promWALErrors])

	return res, nil
}
