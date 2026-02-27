package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoad_Defaults(t *testing.T) {
	// Minimal config — only server_endpoint for agent side; server section absent.
	p := writeConfig(t, `agent:
  server_endpoint: "localhost:50051"
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.GRPCPort != DefaultGRPCPort {
		t.Errorf("grpc_port: got %d, want %d", cfg.Server.GRPCPort, DefaultGRPCPort)
	}
	if cfg.Server.HTTPPort != DefaultHTTPPort {
		t.Errorf("http_port: got %d, want %d", cfg.Server.HTTPPort, DefaultHTTPPort)
	}
	if cfg.Server.Snapshot.TTL != DefaultSnapshotTTL {
		t.Errorf("snapshot.ttl: got %v, want %v", cfg.Server.Snapshot.TTL, DefaultSnapshotTTL)
	}
}

func TestLoad_FullServer(t *testing.T) {
	p := writeConfig(t, `server:
  grpc_port: 9090
  http_port: 9091
  auth:
    mode: apikey
    key_env: MY_KEY
    header: x-obs-key
  snapshot:
    ttl: 10m
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.GRPCPort != 9090 {
		t.Errorf("grpc_port: got %d, want 9090", cfg.Server.GRPCPort)
	}
	if cfg.Server.Auth.Mode != "apikey" {
		t.Errorf("auth.mode: got %q, want apikey", cfg.Server.Auth.Mode)
	}
	if cfg.Server.Auth.EffectiveHeader() != "x-obs-key" {
		t.Errorf("header: got %q, want x-obs-key", cfg.Server.Auth.EffectiveHeader())
	}
	if cfg.Server.Snapshot.TTL != 10*time.Minute {
		t.Errorf("snapshot.ttl: got %v, want 10m", cfg.Server.Snapshot.TTL)
	}
}

func TestLoad_DefaultHeader(t *testing.T) {
	p := writeConfig(t, `server:
  auth:
    mode: apikey
    key_env: K
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if h := cfg.Server.Auth.EffectiveHeader(); h != "x-api-key" {
		t.Errorf("EffectiveHeader: got %q, want x-api-key", h)
	}
}

func TestLoad_KeyEnvResolution(t *testing.T) {
	t.Setenv("TEST_SERVER_KEY", "supersecret")
	p := writeConfig(t, `server:
  auth:
    mode: apikey
    key_env: TEST_SERVER_KEY
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if k := cfg.Server.Auth.Key(); k != "supersecret" {
		t.Errorf("Key(): got %q, want supersecret", k)
	}
}

func TestLoad_UnknownAuthMode(t *testing.T) {
	p := writeConfig(t, `server:
  auth:
    mode: oauth2
`)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for unknown auth mode, got nil")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
