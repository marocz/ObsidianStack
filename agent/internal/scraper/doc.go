// Package scraper provides scrapers for each supported pipeline component.
// Each scraper polls a component's Prometheus metrics endpoint and returns a
// ScrapeResult containing raw counter values (received/dropped per signal type
// plus component-specific extras). The compute engine derives rates and health
// scores from these results.
//
// Implemented scrapers: OTel Collector (otel.go), Prometheus (prometheus.go),
// Loki (loki.go). Factory: New(config.Source) returns the correct Scraper.
//
// Authentication (mTLS, API key, bearer token) is handled by the shared
// authRoundTripper in base.go; individual scrapers receive a pre-configured
// *http.Client from New().
package scraper
