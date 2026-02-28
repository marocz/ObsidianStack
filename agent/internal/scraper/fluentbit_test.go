package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

const fluentbitMetricsJSON = `{
  "input": {
    "tail.0": {"records": 10000, "bytes": 5120000},
    "systemd.1": {"records": 2000,  "bytes": 800000}
  },
  "filter": {
    "grep.0":   {"add_records": 11500, "drop_records": 500},
    "modify.1": {"add_records": 11000, "drop_records": 100}
  },
  "output": {
    "es.0": {
      "proc_records": 10000, "proc_bytes": 5000000,
      "errors": 5, "retries": 20, "retried_failed": 3
    },
    "forward.1": {
      "proc_records": 1700, "proc_bytes": 680000,
      "errors": 0, "retries": 0, "retried_failed": 0
    }
  }
}`

func newFBScraper(t *testing.T, body string, status int) (*fluentbitScraper, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/metrics" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &fluentbitScraper{
		src:    config.Source{ID: "fb-test", Type: "fluentbit", Endpoint: srv.URL},
		client: srv.Client(),
	}, srv
}

func TestFluentBitScraper_Received(t *testing.T) {
	s, _ := newFBScraper(t, fluentbitMetricsJSON, http.StatusOK)
	res, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape error: %v", err)
	}
	if res.Err != nil {
		t.Fatalf("res.Err: %v", res.Err)
	}
	// input: 10000 + 2000 = 12000
	if got := res.Received["logs"]; got != 12000 {
		t.Errorf("Received[logs] = %.0f, want 12000", got)
	}
}

func TestFluentBitScraper_Dropped(t *testing.T) {
	s, _ := newFBScraper(t, fluentbitMetricsJSON, http.StatusOK)
	res, _ := s.Scrape(context.Background())
	// retried_failed: 3+0=3; filter_drop_records: 500+100=600 → total 603
	if got := res.Dropped["logs"]; got != 603 {
		t.Errorf("Dropped[logs] = %.0f, want 603", got)
	}
}

func TestFluentBitScraper_ExtraFields(t *testing.T) {
	s, _ := newFBScraper(t, fluentbitMetricsJSON, http.StatusOK)
	res, _ := s.Scrape(context.Background())

	cases := map[string]float64{
		"input_records":         12000,
		"input_bytes":           5920000,
		"output_proc_records":   11700,
		"output_proc_bytes":     5680000,
		"output_errors":         5,
		"output_retries":        20,
		"output_retried_failed": 3,
		"filter_drop_records":   600,
	}
	for k, want := range cases {
		if got := res.Extra[k]; got != want {
			t.Errorf("Extra[%q] = %.0f, want %.0f", k, got, want)
		}
	}
}

func TestFluentBitScraper_SourceType(t *testing.T) {
	s, _ := newFBScraper(t, fluentbitMetricsJSON, http.StatusOK)
	res, _ := s.Scrape(context.Background())
	if res.SourceType != "fluentbit" {
		t.Errorf("SourceType = %q, want fluentbit", res.SourceType)
	}
}

func TestFluentBitScraper_ConnectFailure(t *testing.T) {
	s := &fluentbitScraper{
		src:    config.Source{ID: "fb-down", Type: "fluentbit", Endpoint: "http://127.0.0.1:1"},
		client: &http.Client{},
	}
	res, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() should not return err for connectivity failure, got: %v", err)
	}
	if res.Err == nil {
		t.Fatal("res.Err should be set when endpoint is unreachable")
	}
}

func TestFluentBitScraper_Non200(t *testing.T) {
	s, _ := newFBScraper(t, "", http.StatusUnauthorized)
	res, _ := s.Scrape(context.Background())
	if res.Err == nil {
		t.Fatal("res.Err should be set for non-200 response")
	}
}

func TestFluentBitScraper_InvalidJSON(t *testing.T) {
	s, _ := newFBScraper(t, `{not valid json`, http.StatusOK)
	res, _ := s.Scrape(context.Background())
	if res.Err == nil {
		t.Fatal("res.Err should be set for invalid JSON")
	}
}

func TestFluentBitScraper_EmptyPlugins(t *testing.T) {
	// No plugins running — all zeros, no panic.
	s, _ := newFBScraper(t, `{"input":{},"filter":{},"output":{}}`, http.StatusOK)
	res, _ := s.Scrape(context.Background())
	if res.Err != nil {
		t.Fatalf("res.Err: %v", res.Err)
	}
	if res.Received["logs"] != 0 {
		t.Errorf("Received[logs] = %.0f, want 0", res.Received["logs"])
	}
}

func TestFluentBitScraper_EndpointPathAppended(t *testing.T) {
	// Verify the scraper appends /api/v1/metrics even if the endpoint has a trailing slash.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"input":{},"filter":{},"output":{}}`))
	}))
	t.Cleanup(srv.Close)

	s := &fluentbitScraper{
		src:    config.Source{ID: "x", Endpoint: srv.URL + "/"},
		client: srv.Client(),
	}
	s.Scrape(context.Background()) //nolint:errcheck

	if gotPath != "/api/v1/metrics" {
		t.Errorf("request path = %q, want /api/v1/metrics", gotPath)
	}
}

func TestNew_FluentBitType(t *testing.T) {
	src := config.Source{ID: "fb", Type: "fluentbit", Endpoint: "http://localhost:2020"}
	scraper, err := New(src)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if scraper == nil {
		t.Fatal("New() returned nil scraper")
	}
}
