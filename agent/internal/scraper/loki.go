package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// Loki internal metric names we track.
const (
	// Lines received by the distributor from push requests.
	lokiLinesReceived = "loki_distributor_lines_received_total"

	// Bytes received by the distributor from push requests.
	lokiBytesReceived = "loki_distributor_bytes_received_total"

	// Lines successfully flushed to storage by ingesters.
	lokiLinesFlushed = "loki_ingester_chunks_flushed_total"

	// Flush failures — chunks that could not be written to the store.
	lokiFlushErrors = "loki_ingester_flush_failures_total"

	// Ring health — number of tokens this instance owns in the hash ring.
	// Only present in microservice mode (distributor/ingester); absent in
	// monolithic single-binary mode. Zero here means the ring metric is
	// unavailable, not that the ring is unhealthy.
	lokiRingTokens = "cortex_ring_tokens_owned"

	// Replication set size — how many ingesters each write is replicated to.
	lokiRingReplication = "cortex_ring_replication_factor"

	// Ingester ingestion rate — samples/sec being appended.
	lokiIngestionRate = "loki_ingester_ingestion_rate_bytes"
)

type lokiScraper struct {
	src    config.Source
	client *http.Client
}

// Scrape fetches Loki's /metrics endpoint and extracts log ingestion
// and storage health data.
//
// All signal data is reported under the "logs" signal type.
// The ring health metrics (cortex_ring_*) are only present in microservice
// mode; in monolithic mode they will be 0 in Extra, which is not an error.
func (s *lokiScraper) Scrape(ctx context.Context) (*ScrapeResult, error) {
	res := newResult(s.src.ID, "loki")

	mfs, err := fetchMetrics(ctx, s.client, s.src.Endpoint)
	if err != nil {
		res.Err = fmt.Errorf("loki scrape %q: %w", s.src.ID, err)
		slog.Warn("scraper: loki fetch failed", "source", s.src.ID, "err", err)
		return res, nil
	}

	linesReceived := sumFamily(mfs[lokiLinesReceived])
	linesFlushed := sumFamily(mfs[lokiLinesFlushed])
	flushErrors := sumFamily(mfs[lokiFlushErrors])

	// Received = lines that entered the distributor.
	// Dropped = flush failures at the ingester layer.
	res.Received["logs"] = linesReceived
	res.Dropped["logs"] = flushErrors

	res.Extra["lines_received"] = linesReceived
	res.Extra["bytes_received"] = sumFamily(mfs[lokiBytesReceived])
	res.Extra["lines_flushed"] = linesFlushed
	res.Extra["flush_errors"] = flushErrors
	res.Extra["ring_tokens"] = sumFamily(mfs[lokiRingTokens])
	res.Extra["ring_replication"] = sumFamily(mfs[lokiRingReplication])
	res.Extra["ingestion_rate_bytes"] = sumFamily(mfs[lokiIngestionRate])

	return res, nil
}
