package scraper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

const defaultScrapeTimeout = 10 * time.Second

// ScrapeResult is the normalized output of one scrape cycle for a single source.
// Counter fields hold raw totals — not per-minute rates. The compute engine
// maintains the previous result and derives rates from the delta.
type ScrapeResult struct {
	SourceID   string
	SourceType string
	ScrapedAt  time.Time

	// Received holds the total count of items accepted per signal type.
	// Keys are canonical signal names: "metrics", "logs", "traces".
	Received map[string]float64

	// Dropped holds the total count of items dropped per signal type.
	// Includes exporter send failures and processor drops.
	Dropped map[string]float64

	// Extra holds component-specific metrics not covered by Received/Dropped.
	// Examples: "queue_capacity", "queue_pending", "ring_tokens".
	Extra map[string]float64

	// Err is non-nil if the scrape itself failed (connectivity, auth, parse).
	// The compute engine treats a non-nil Err as an Unknown health state.
	Err error
}

// Scraper is the common interface implemented by every pipeline component scraper.
type Scraper interface {
	Scrape(ctx context.Context) (*ScrapeResult, error)
}

// New returns the appropriate Scraper for the given source configuration.
// It builds the HTTP client once and reuses it across scrape calls.
func New(src config.Source) (Scraper, error) {
	client, err := buildHTTPClient(src)
	if err != nil {
		return nil, fmt.Errorf("scraper %q: build http client: %w", src.ID, err)
	}
	switch src.Type {
	case "otelcol":
		return &otelScraper{src: src, client: client}, nil
	case "prometheus":
		return &promScraper{src: src, client: client}, nil
	case "loki":
		return &lokiScraper{src: src, client: client}, nil
	case "fluentbit":
		return &fluentbitScraper{src: src, client: client}, nil
	default:
		return nil, fmt.Errorf("scraper: unsupported type %q", src.Type)
	}
}

// authRoundTripper injects authentication headers into every outgoing request.
type authRoundTripper struct {
	base http.RoundTripper
	src  config.Source
}

func (t *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	switch t.src.Auth.Mode {
	case "apikey":
		req = req.Clone(req.Context())
		req.Header.Set(t.src.Auth.Header, t.src.Auth.Key())
	case "bearer":
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+t.src.Auth.Token())
	case "basic":
		req = req.Clone(req.Context())
		req.SetBasicAuth(t.src.Auth.Username, t.src.Auth.Password())
	}
	return t.base.RoundTrip(req)
}

// buildHTTPClient constructs an http.Client for the source's auth and TLS settings.
func buildHTTPClient(src config.Source) (*http.Client, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: src.TLS.InsecureSkipVerify, //nolint:gosec // user-configured
	}

	if src.Auth.Mode == "mtls" {
		cert, err := tls.LoadX509KeyPair(src.Auth.CertFile, src.Auth.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}

		if src.Auth.CAFile != "" {
			caPEM, err := os.ReadFile(src.Auth.CAFile)
			if err != nil {
				return nil, fmt.Errorf("read ca file: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, fmt.Errorf("no valid certs found in ca file %q", src.Auth.CAFile)
			}
			tlsCfg.RootCAs = pool
		}
	}

	transport := &authRoundTripper{
		base: &http.Transport{TLSClientConfig: tlsCfg},
		src:  src,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   defaultScrapeTimeout,
	}, nil
}

// fetchMetrics performs an HTTP GET to url and returns parsed metric families.
func fetchMetrics(ctx context.Context, client *http.Client, url string) (map[string]*dto.MetricFamily, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", string(expfmt.NewFormat(expfmt.TypeTextPlain)))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return parseMetrics(resp.Body)
}

// parseMetrics decodes a Prometheus text exposition from r into metric families.
// A partial result with a non-fatal parse warning is still returned successfully.
func parseMetrics(r io.Reader) (map[string]*dto.MetricFamily, error) {
	var parser expfmt.TextParser
	mfs, err := parser.TextToMetricFamilies(r)
	if err != nil && len(mfs) == 0 {
		return nil, fmt.Errorf("parse prometheus text: %w", err)
	}
	// Non-empty result with a non-nil err means partial parse (trailing lines,
	// format warnings). Treat as success.
	return mfs, nil
}

// sumFamily adds up all counter, gauge, or untyped values in a MetricFamily.
// Returns 0 if mf is nil (metric not present in the scrape).
func sumFamily(mf *dto.MetricFamily) float64 {
	if mf == nil {
		return 0
	}
	var total float64
	for _, m := range mf.GetMetric() {
		switch {
		case m.Counter != nil:
			total += m.Counter.GetValue()
		case m.Gauge != nil:
			total += m.Gauge.GetValue()
		case m.Untyped != nil:
			total += m.Untyped.GetValue()
		}
	}
	return total
}

// newResult initialises an empty ScrapeResult with all maps allocated.
func newResult(sourceID, sourceType string) *ScrapeResult {
	return &ScrapeResult{
		SourceID:   sourceID,
		SourceType: sourceType,
		ScrapedAt:  time.Now().UTC(),
		Received:   make(map[string]float64),
		Dropped:    make(map[string]float64),
		Extra:      make(map[string]float64),
	}
}
