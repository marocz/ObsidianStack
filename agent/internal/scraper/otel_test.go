package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// otelMetrics is a realistic sample of OTel Collector internal metrics.
const otelMetrics = `
# HELP otelcol_receiver_accepted_spans Number of spans successfully pushed into the pipeline.
# TYPE otelcol_receiver_accepted_spans counter
otelcol_receiver_accepted_spans{receiver="otlp",transport="grpc"} 10000
otelcol_receiver_accepted_spans{receiver="jaeger",transport="thrift"} 2000

# HELP otelcol_receiver_refused_spans Number of spans that could not be pushed into the pipeline.
# TYPE otelcol_receiver_refused_spans counter
otelcol_receiver_refused_spans{receiver="otlp",transport="grpc"} 50

# HELP otelcol_exporter_sent_spans Number of spans successfully sent to destination.
# TYPE otelcol_exporter_sent_spans counter
otelcol_exporter_sent_spans{exporter="otlp"} 11800

# HELP otelcol_exporter_send_failed_spans Number of spans that failed to be sent.
# TYPE otelcol_exporter_send_failed_spans counter
otelcol_exporter_send_failed_spans{exporter="otlp"} 150

# HELP otelcol_processor_dropped_spans Number of spans that were dropped.
# TYPE otelcol_processor_dropped_spans counter
otelcol_processor_dropped_spans{processor="batch"} 25

# HELP otelcol_receiver_accepted_metric_points Number of metric points successfully pushed.
# TYPE otelcol_receiver_accepted_metric_points counter
otelcol_receiver_accepted_metric_points{receiver="prometheus",transport="http"} 5000

# HELP otelcol_exporter_send_failed_metric_points Number of metric points that failed.
# TYPE otelcol_exporter_send_failed_metric_points counter
otelcol_exporter_send_failed_metric_points{exporter="prometheusremotewrite"} 200

# HELP otelcol_receiver_accepted_log_records Number of log records successfully pushed.
# TYPE otelcol_receiver_accepted_log_records counter
otelcol_receiver_accepted_log_records{receiver="filelog"} 8000

# HELP otelcol_exporter_send_failed_log_records Number of log records that failed.
# TYPE otelcol_exporter_send_failed_log_records counter
otelcol_exporter_send_failed_log_records{exporter="loki"} 80

# HELP otelcol_exporter_queue_size Current size of the retry queue.
# TYPE otelcol_exporter_queue_size gauge
otelcol_exporter_queue_size{exporter="otlp"} 42

# HELP otelcol_exporter_queue_capacity Capacity of the retry queue.
# TYPE otelcol_exporter_queue_capacity gauge
otelcol_exporter_queue_capacity{exporter="otlp"} 1000
`

func TestOTelScraper_Scrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(otelMetrics))
	}))
	defer srv.Close()

	src := config.Source{
		ID:       "otel-test",
		Type:     "otelcol",
		Endpoint: srv.URL,
		Auth:     config.AuthConfig{Mode: "none"},
	}
	scraper := &otelScraper{src: src, client: srv.Client()}

	res, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}
	if res.Err != nil {
		t.Fatalf("res.Err = %v", res.Err)
	}

	// traces: 10000 + 2000 accepted, 150 failed + 25 processor = 175 dropped
	if got := res.Received["traces"]; got != 12000 {
		t.Errorf("Received[traces] = %v, want 12000", got)
	}
	if got := res.Dropped["traces"]; got != 175 {
		t.Errorf("Dropped[traces] = %v, want 175", got)
	}

	// metrics: 5000 accepted, 200 failed
	if got := res.Received["metrics"]; got != 5000 {
		t.Errorf("Received[metrics] = %v, want 5000", got)
	}
	if got := res.Dropped["metrics"]; got != 200 {
		t.Errorf("Dropped[metrics] = %v, want 200", got)
	}

	// logs: 8000 accepted, 80 failed
	if got := res.Received["logs"]; got != 8000 {
		t.Errorf("Received[logs] = %v, want 8000", got)
	}
	if got := res.Dropped["logs"]; got != 80 {
		t.Errorf("Dropped[logs] = %v, want 80", got)
	}

	// queue
	if got := res.Extra["exporter_queue_size"]; got != 42 {
		t.Errorf("Extra[exporter_queue_size] = %v, want 42", got)
	}
	if got := res.Extra["exporter_queue_capacity"]; got != 1000 {
		t.Errorf("Extra[exporter_queue_capacity] = %v, want 1000", got)
	}
}

func TestOTelScraper_MultiLabel_Accumulation(t *testing.T) {
	// Two receiver instances for traces — both should be summed.
	body := `
otelcol_receiver_accepted_spans{receiver="otlp"} 100
otelcol_receiver_accepted_spans{receiver="jaeger"} 200
otelcol_exporter_send_failed_spans{exporter="otlp"} 10
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	s := &otelScraper{src: config.Source{ID: "x", Type: "otelcol", Endpoint: srv.URL}, client: srv.Client()}
	res, _ := s.Scrape(context.Background())

	if got := res.Received["traces"]; got != 300 {
		t.Errorf("Received[traces] with two labels = %v, want 300", got)
	}
	if got := res.Dropped["traces"]; got != 10 {
		t.Errorf("Dropped[traces] = %v, want 10", got)
	}
}

func TestOTelScraper_ConnectFailure(t *testing.T) {
	src := config.Source{
		ID:       "otel-down",
		Type:     "otelcol",
		Endpoint: "http://127.0.0.1:1", // nothing listening
		Auth:     config.AuthConfig{Mode: "none"},
	}
	client := &http.Client{}
	s := &otelScraper{src: src, client: client}

	res, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() should not return err for connectivity failure, got: %v", err)
	}
	if res.Err == nil {
		t.Fatal("res.Err should be set when endpoint is unreachable")
	}
}

func TestOTelScraper_Non200Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	s := &otelScraper{src: config.Source{ID: "x", Endpoint: srv.URL}, client: srv.Client()}
	res, _ := s.Scrape(context.Background())
	if res.Err == nil {
		t.Fatal("res.Err should be set for 401 response")
	}
}

func TestOTelScraper_APIKeyAuth(t *testing.T) {
	const wantKey = "test-secret-key"
	var gotKey string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# empty\n"))
	}))
	defer srv.Close()

	t.Setenv("TEST_OTEL_KEY", wantKey)
	src := config.Source{
		ID:       "otel-apikey",
		Type:     "otelcol",
		Endpoint: srv.URL,
		Auth:     config.AuthConfig{Mode: "apikey", Header: "X-API-Key", KeyEnv: "TEST_OTEL_KEY"},
	}

	client, err := buildHTTPClient(src)
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	s := &otelScraper{src: src, client: client}
	s.Scrape(context.Background()) //nolint:errcheck

	if gotKey != wantKey {
		t.Errorf("X-API-Key header = %q, want %q", gotKey, wantKey)
	}
}

func TestOTelScraper_BearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("# empty\n"))
	}))
	defer srv.Close()

	t.Setenv("TEST_OTEL_TOKEN", "mytoken")
	src := config.Source{
		ID:       "otel-bearer",
		Type:     "otelcol",
		Endpoint: srv.URL,
		Auth:     config.AuthConfig{Mode: "bearer", TokenEnv: "TEST_OTEL_TOKEN"},
	}

	client, err := buildHTTPClient(src)
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	s := &otelScraper{src: src, client: client}
	s.Scrape(context.Background()) //nolint:errcheck

	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer mytoken")
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	src := config.Source{ID: "x", Type: "jaeger", Endpoint: "http://localhost:14269"}
	_, err := New(src)
	if err == nil {
		t.Fatal("New() with unsupported type should return error")
	}
}
