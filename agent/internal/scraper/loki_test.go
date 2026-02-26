package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// lokiMetrics is a realistic sample of Loki distributor + ingester metrics.
const lokiMetrics = `
# HELP loki_distributor_lines_received_total The number of lines received.
# TYPE loki_distributor_lines_received_total counter
loki_distributor_lines_received_total{tenant="prod"} 3000000
loki_distributor_lines_received_total{tenant="staging"} 500000

# HELP loki_distributor_bytes_received_total The number of bytes received.
# TYPE loki_distributor_bytes_received_total counter
loki_distributor_bytes_received_total{tenant="prod"} 450000000
loki_distributor_bytes_received_total{tenant="staging"} 75000000

# HELP loki_ingester_chunks_flushed_total Total flushed chunks.
# TYPE loki_ingester_chunks_flushed_total counter
loki_ingester_chunks_flushed_total{reason="full"} 12000
loki_ingester_chunks_flushed_total{reason="idle"} 800

# HELP loki_ingester_flush_failures_total Total number of failed flushed chunks.
# TYPE loki_ingester_flush_failures_total counter
loki_ingester_flush_failures_total 250

# HELP cortex_ring_tokens_owned The number of tokens owned in the ring.
# TYPE cortex_ring_tokens_owned gauge
cortex_ring_tokens_owned{name="ingester"} 128

# HELP cortex_ring_replication_factor The configured replication factor for the ring.
# TYPE cortex_ring_replication_factor gauge
cortex_ring_replication_factor{name="ingester"} 3

# HELP loki_ingester_ingestion_rate_bytes Per-second data rate.
# TYPE loki_ingester_ingestion_rate_bytes gauge
loki_ingester_ingestion_rate_bytes{tenant="prod"} 152340
`

func TestLokiScraper_Scrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(lokiMetrics))
	}))
	defer srv.Close()

	s := &lokiScraper{
		src:    config.Source{ID: "loki-test", Type: "loki", Endpoint: srv.URL},
		client: srv.Client(),
	}

	res, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}
	if res.Err != nil {
		t.Fatalf("res.Err = %v", res.Err)
	}

	// Received = prod + staging lines
	if got := res.Received["logs"]; got != 3500000 {
		t.Errorf("Received[logs] = %v, want 3500000", got)
	}
	// Dropped = flush failures
	if got := res.Dropped["logs"]; got != 250 {
		t.Errorf("Dropped[logs] = %v, want 250", got)
	}

	// bytes received
	if got := res.Extra["bytes_received"]; got != 525000000 {
		t.Errorf("Extra[bytes_received] = %v, want 525000000", got)
	}
	// lines flushed: full + idle
	if got := res.Extra["lines_flushed"]; got != 12800 {
		t.Errorf("Extra[lines_flushed] = %v, want 12800", got)
	}
	// ring tokens
	if got := res.Extra["ring_tokens"]; got != 128 {
		t.Errorf("Extra[ring_tokens] = %v, want 128", got)
	}
	// ring replication factor
	if got := res.Extra["ring_replication"]; got != 3 {
		t.Errorf("Extra[ring_replication] = %v, want 3", got)
	}
}

func TestLokiScraper_MonolithicMode_NoRingMetrics(t *testing.T) {
	// Monolithic Loki doesn't expose ring metrics — values should be 0, not an error.
	body := `
loki_distributor_lines_received_total{tenant="prod"} 100
loki_distributor_bytes_received_total{tenant="prod"} 5000
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := &lokiScraper{src: config.Source{ID: "loki-mono", Endpoint: srv.URL}, client: srv.Client()}
	res, _ := s.Scrape(context.Background())

	if res.Err != nil {
		t.Fatalf("res.Err should be nil for monolithic mode, got: %v", res.Err)
	}
	if got := res.Extra["ring_tokens"]; got != 0 {
		t.Errorf("ring_tokens in monolithic mode = %v, want 0", got)
	}
	if got := res.Received["logs"]; got != 100 {
		t.Errorf("Received[logs] = %v, want 100", got)
	}
}

func TestLokiScraper_ConnectFailure(t *testing.T) {
	s := &lokiScraper{
		src:    config.Source{ID: "loki-down", Endpoint: "http://127.0.0.1:1"},
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

func TestLokiScraper_Non200Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s := &lokiScraper{src: config.Source{ID: "loki-403", Endpoint: srv.URL}, client: srv.Client()}
	res, _ := s.Scrape(context.Background())
	if res.Err == nil {
		t.Fatal("res.Err should be set for 403 response")
	}
}

func TestSumFamily_Nil(t *testing.T) {
	if got := sumFamily(nil); got != 0 {
		t.Errorf("sumFamily(nil) = %v, want 0", got)
	}
}
