package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Valid(t *testing.T) {
	yaml := `
agent:
  server_endpoint: "localhost:50051"
  scrape_interval: 10s
  ship_interval: 5s
  buffer_size: 500
  sources:
    - id: otel-prod
      type: otelcol
      endpoint: "http://localhost:8888/metrics"
      auth:
        mode: none
`
	cfg := loadFromString(t, yaml)

	if cfg.Agent.ServerEndpoint != "localhost:50051" {
		t.Errorf("server_endpoint: got %q", cfg.Agent.ServerEndpoint)
	}
	if cfg.Agent.ScrapeInterval != 10*time.Second {
		t.Errorf("scrape_interval: got %v", cfg.Agent.ScrapeInterval)
	}
	if cfg.Agent.BufferSize != 500 {
		t.Errorf("buffer_size: got %d", cfg.Agent.BufferSize)
	}
	if len(cfg.Agent.Sources) != 1 {
		t.Fatalf("sources: got %d, want 1", len(cfg.Agent.Sources))
	}
	src := cfg.Agent.Sources[0]
	if src.ID != "otel-prod" {
		t.Errorf("source id: got %q", src.ID)
	}
	if src.Type != "otelcol" {
		t.Errorf("source type: got %q", src.Type)
	}
}

func TestLoad_Defaults(t *testing.T) {
	yaml := `
agent:
  server_endpoint: "localhost:50051"
  sources:
    - id: prom
      type: prometheus
      endpoint: "http://localhost:9090/metrics"
`
	cfg := loadFromString(t, yaml)

	if cfg.Agent.ScrapeInterval != DefaultScrapeInterval {
		t.Errorf("default scrape_interval: got %v, want %v", cfg.Agent.ScrapeInterval, DefaultScrapeInterval)
	}
	if cfg.Agent.ShipInterval != DefaultShipInterval {
		t.Errorf("default ship_interval: got %v, want %v", cfg.Agent.ShipInterval, DefaultShipInterval)
	}
	if cfg.Agent.BufferSize != DefaultBufferSize {
		t.Errorf("default buffer_size: got %d, want %d", cfg.Agent.BufferSize, DefaultBufferSize)
	}
	if cfg.Server.GRPCPort != DefaultGRPCPort {
		t.Errorf("default grpc_port: got %d, want %d", cfg.Server.GRPCPort, DefaultGRPCPort)
	}
}

func TestLoad_MissingServerEndpoint(t *testing.T) {
	yaml := `
agent:
  sources:
    - id: prom
      type: prometheus
      endpoint: "http://localhost:9090/metrics"
`
	_, err := loadStringErr(t, yaml)
	if err == nil {
		t.Fatal("expected error for missing server_endpoint, got nil")
	}
}

func TestLoad_UnknownSourceType(t *testing.T) {
	yaml := `
agent:
  server_endpoint: "localhost:50051"
  sources:
    - id: mystery
      type: unknowntype
      endpoint: "http://localhost:9999/metrics"
`
	_, err := loadStringErr(t, yaml)
	if err == nil {
		t.Fatal("expected error for unknown source type, got nil")
	}
}

func TestLoad_UnknownAuthMode(t *testing.T) {
	yaml := `
agent:
  server_endpoint: "localhost:50051"
  sources:
    - id: otel
      type: otelcol
      endpoint: "http://localhost:8888/metrics"
      auth:
        mode: magictoken
`
	_, err := loadStringErr(t, yaml)
	if err == nil {
		t.Fatal("expected error for unknown auth mode, got nil")
	}
}

func TestAuthConfig_Key(t *testing.T) {
	t.Setenv("TEST_API_KEY", "supersecret")
	a := AuthConfig{Mode: "apikey", KeyEnv: "TEST_API_KEY"}
	if got := a.Key(); got != "supersecret" {
		t.Errorf("Key(): got %q, want %q", got, "supersecret")
	}
}

func TestAuthConfig_Key_Empty(t *testing.T) {
	a := AuthConfig{Mode: "apikey"}
	if got := a.Key(); got != "" {
		t.Errorf("Key() with no KeyEnv: got %q, want empty", got)
	}
}

func TestAuthConfig_Token(t *testing.T) {
	t.Setenv("TEST_BEARER_TOKEN", "mytoken")
	a := AuthConfig{Mode: "bearer", TokenEnv: "TEST_BEARER_TOKEN"}
	if got := a.Token(); got != "mytoken" {
		t.Errorf("Token(): got %q, want %q", got, "mytoken")
	}
}

func TestWebhookConfig_URL(t *testing.T) {
	t.Setenv("TEAMS_URL", "https://teams.example.com/webhook")
	w := WebhookConfig{Type: "teams", URLEnv: "TEAMS_URL"}
	if got := w.URL(); got != "https://teams.example.com/webhook" {
		t.Errorf("URL(): got %q", got)
	}
}

func TestLoad_MultipleAuthModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{"mtls", "mtls"},
		{"apikey", "apikey"},
		{"bearer", "bearer"},
		{"none", "none"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := `
agent:
  server_endpoint: "localhost:50051"
  sources:
    - id: src
      type: otelcol
      endpoint: "http://localhost:8888/metrics"
      auth:
        mode: ` + tc.mode + `
`
			cfg := loadFromString(t, yaml)
			if cfg.Agent.Sources[0].Auth.Mode != tc.mode {
				t.Errorf("auth mode: got %q, want %q", cfg.Agent.Sources[0].Auth.Mode, tc.mode)
			}
		})
	}
}

// loadFromString writes yaml to a temp file and calls Load, failing on error.
func loadFromString(t *testing.T, content string) *Config {
	t.Helper()
	cfg, err := loadStringErr(t, content)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	return cfg
}

// loadStringErr writes yaml to a temp file and calls Load, returning any error.
func loadStringErr(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return Load(path)
}
