package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// fluentbitMetrics is the JSON shape returned by Fluent Bit's /api/v1/metrics.
type fluentbitMetrics struct {
	Input  map[string]fbInput  `json:"input"`
	Filter map[string]fbFilter `json:"filter"`
	Output map[string]fbOutput `json:"output"`
}

type fbInput struct {
	Records uint64 `json:"records"`
	Bytes   uint64 `json:"bytes"`
}

type fbFilter struct {
	AddRecords  uint64 `json:"add_records"`
	DropRecords uint64 `json:"drop_records"`
}

type fbOutput struct {
	ProcRecords   uint64 `json:"proc_records"`
	ProcBytes     uint64 `json:"proc_bytes"`
	Errors        uint64 `json:"errors"`
	Retries       uint64 `json:"retries"`
	RetriedFailed uint64 `json:"retried_failed"`
}

type fluentbitScraper struct {
	src    config.Source
	client *http.Client
}

// Scrape fetches Fluent Bit's /api/v1/metrics JSON endpoint and extracts
// log pipeline health data.
//
// Received = total records ingested across all input plugins.
// Dropped  = records permanently lost: output retried_failed (max retries
//
//	exhausted) + records dropped by filter plugins.
//
// Extra fields (counters — compute engine derives _pm rates):
//
//	input_records, input_bytes
//	output_proc_records, output_proc_bytes
//	output_errors, output_retries, output_retried_failed
//	filter_drop_records
func (s *fluentbitScraper) Scrape(ctx context.Context) (*ScrapeResult, error) {
	res := newResult(s.src.ID, "fluentbit")

	url := strings.TrimRight(s.src.Endpoint, "/") + "/api/v1/metrics"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		res.Err = fmt.Errorf("fluentbit scrape %q: build request: %w", s.src.ID, err)
		return res, nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		res.Err = fmt.Errorf("fluentbit scrape %q: %w", s.src.ID, err)
		slog.Warn("scraper: fluentbit fetch failed", "source", s.src.ID, "err", err)
		return res, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Errorf("fluentbit scrape %q: unexpected status %d", s.src.ID, resp.StatusCode)
		return res, nil
	}

	var m fluentbitMetrics
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		res.Err = fmt.Errorf("fluentbit scrape %q: decode JSON: %w", s.src.ID, err)
		return res, nil
	}

	// Sum across all input plugins.
	var inputRecords, inputBytes float64
	for _, p := range m.Input {
		inputRecords += float64(p.Records)
		inputBytes += float64(p.Bytes)
	}

	// Sum across all filter plugins.
	var filterDropped float64
	for _, p := range m.Filter {
		filterDropped += float64(p.DropRecords)
	}

	// Sum across all output plugins.
	var outProc, outBytes, outErrors, outRetries, outRetriedFailed float64
	for _, p := range m.Output {
		outProc += float64(p.ProcRecords)
		outBytes += float64(p.ProcBytes)
		outErrors += float64(p.Errors)
		outRetries += float64(p.Retries)
		outRetriedFailed += float64(p.RetriedFailed)
	}

	// The compute engine uses: drop_pct = dropped / (received + dropped)
	// so Received must be records that successfully exited (output_proc_records),
	// not records that entered (input_records). Using input here would halve the
	// computed drop_pct when 100% of records are filtered/lost.
	res.Received["logs"] = outProc
	res.Dropped["logs"] = outRetriedFailed + filterDropped

	res.Extra["input_records"] = inputRecords
	res.Extra["input_bytes"] = inputBytes
	res.Extra["output_proc_records"] = outProc
	res.Extra["output_proc_bytes"] = outBytes
	res.Extra["output_errors"] = outErrors
	res.Extra["output_retries"] = outRetries
	res.Extra["output_retried_failed"] = outRetriedFailed
	res.Extra["filter_drop_records"] = filterDropped

	return res, nil
}
