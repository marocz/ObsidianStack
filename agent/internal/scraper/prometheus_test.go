package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// promMetrics is a realistic subset of Prometheus's own /metrics output.
const promMetrics = `
# HELP prometheus_tsdb_head_samples_appended_total Total number of appended samples.
# TYPE prometheus_tsdb_head_samples_appended_total counter
prometheus_tsdb_head_samples_appended_total{type="float"} 4500000
prometheus_tsdb_head_samples_appended_total{type="histogram"} 120000

# HELP prometheus_remote_storage_samples_dropped_total Total number of samples dropped.
# TYPE prometheus_remote_storage_samples_dropped_total counter
prometheus_remote_storage_samples_dropped_total{remote_name="thanos",url="http://thanos:19291"} 1250

# HELP prometheus_remote_storage_succeeded_samples_total Total samples successfully sent.
# TYPE prometheus_remote_storage_succeeded_samples_total counter
prometheus_remote_storage_succeeded_samples_total{remote_name="thanos",url="http://thanos:19291"} 4498750

# HELP prometheus_remote_storage_samples_pending Current number of samples pending send.
# TYPE prometheus_remote_storage_samples_pending gauge
prometheus_remote_storage_samples_pending{remote_name="thanos",url="http://thanos:19291"} 340

# HELP prometheus_remote_storage_queue_capacity Capacity of the queue of samples to be sent.
# TYPE prometheus_remote_storage_queue_capacity gauge
prometheus_remote_storage_queue_capacity{remote_name="thanos",url="http://thanos:19291"} 10000

# HELP prometheus_remote_storage_shards Number of shards used for parallel sending.
# TYPE prometheus_remote_storage_shards gauge
prometheus_remote_storage_shards{remote_name="thanos",url="http://thanos:19291"} 4

# HELP prometheus_tsdb_wal_storage_errors_total Total number of WAL storage errors.
# TYPE prometheus_tsdb_wal_storage_errors_total counter
prometheus_tsdb_wal_storage_errors_total 0
`

func TestPromScraper_Scrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(promMetrics))
	}))
	defer srv.Close()

	s := &promScraper{
		src:    config.Source{ID: "prom-test", Type: "prometheus", Endpoint: srv.URL},
		client: srv.Client(),
	}

	res, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}
	if res.Err != nil {
		t.Fatalf("res.Err = %v", res.Err)
	}

	// Received = float + histogram samples appended
	if got := res.Received["metrics"]; got != 4620000 {
		t.Errorf("Received[metrics] = %v, want 4620000", got)
	}
	// Dropped = samples dropped via remote storage
	if got := res.Dropped["metrics"]; got != 1250 {
		t.Errorf("Dropped[metrics] = %v, want 1250", got)
	}

	if got := res.Extra["samples_sent"]; got != 4498750 {
		t.Errorf("Extra[samples_sent] = %v, want 4498750", got)
	}
	if got := res.Extra["queue_pending"]; got != 340 {
		t.Errorf("Extra[queue_pending] = %v, want 340", got)
	}
	if got := res.Extra["queue_capacity"]; got != 10000 {
		t.Errorf("Extra[queue_capacity] = %v, want 10000", got)
	}
	if got := res.Extra["shards_active"]; got != 4 {
		t.Errorf("Extra[shards_active] = %v, want 4", got)
	}
	if got := res.Extra["wal_errors"]; got != 0 {
		t.Errorf("Extra[wal_errors] = %v, want 0", got)
	}
}

func TestPromScraper_MultiRemote_Accumulation(t *testing.T) {
	// Two remote-write destinations — drops should be summed.
	body := `
prometheus_remote_storage_samples_dropped_total{remote_name="thanos"} 100
prometheus_remote_storage_samples_dropped_total{remote_name="cortex"} 50
prometheus_tsdb_head_samples_appended_total{type="float"} 1000
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := &promScraper{src: config.Source{ID: "prom-multi", Endpoint: srv.URL}, client: srv.Client()}
	res, _ := s.Scrape(context.Background())

	if got := res.Dropped["metrics"]; got != 150 {
		t.Errorf("Dropped[metrics] with two remotes = %v, want 150", got)
	}
}

func TestPromScraper_NoRemoteWrite_ZeroDrops(t *testing.T) {
	// Standalone Prometheus with no remote_write config — drop metrics absent.
	body := `
prometheus_tsdb_head_samples_appended_total{type="float"} 9999
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := &promScraper{src: config.Source{ID: "prom-local", Endpoint: srv.URL}, client: srv.Client()}
	res, _ := s.Scrape(context.Background())

	if got := res.Dropped["metrics"]; got != 0 {
		t.Errorf("Dropped[metrics] without remote write = %v, want 0", got)
	}
	if got := res.Received["metrics"]; got != 9999 {
		t.Errorf("Received[metrics] = %v, want 9999", got)
	}
}

func TestPromScraper_ConnectFailure(t *testing.T) {
	s := &promScraper{
		src:    config.Source{ID: "prom-down", Endpoint: "http://127.0.0.1:1"},
		client: &http.Client{},
	}
	res, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() should not return err, got: %v", err)
	}
	if res.Err == nil {
		t.Fatal("res.Err should be set when endpoint is unreachable")
	}
}
