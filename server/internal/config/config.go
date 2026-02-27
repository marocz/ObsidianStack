package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AlertsConfig holds alerting rules and webhook delivery targets.
type AlertsConfig struct {
	Rules    []AlertRule     `yaml:"rules"`
	Webhooks []WebhookConfig `yaml:"webhooks"`
}

// AlertRule defines one threshold-based alert condition.
type AlertRule struct {
	// Name is the human-readable alert identifier, used as the deduplication key.
	Name string `yaml:"name"`

	// Condition is a simple expression: "drop_pct > 10", "strength_score < 60",
	// "cert_days_left < 14", "state == critical".
	Condition string `yaml:"condition"`

	// Severity is one of: critical | warning | info.
	Severity string `yaml:"severity"`

	// Cooldown suppresses re-fires for this duration after an alert fires.
	// Defaults to 15 minutes if zero.
	Cooldown time.Duration `yaml:"cooldown"`
}

// WebhookConfig defines one webhook delivery target.
type WebhookConfig struct {
	// Type is one of: teams | slack | pagerduty | http.
	Type string `yaml:"type"`

	// URLEnv is the name of the environment variable that holds the webhook URL.
	URLEnv string `yaml:"url_env"`
}

// URL returns the webhook URL resolved from the environment.
func (w WebhookConfig) URL() string {
	if w.URLEnv == "" {
		return ""
	}
	return os.Getenv(w.URLEnv)
}

// Default values for the server configuration.
const (
	DefaultGRPCPort    = 50051
	DefaultHTTPPort    = 8080
	DefaultSnapshotTTL = 5 * time.Minute
)

// Config holds the server-side configuration parsed from the `server:` section
// of config.yaml. The `agent:` key in the same file is ignored.
type Config struct {
	Server ServerConfig `yaml:"server"`
}

// ServerConfig holds all server-side settings.
type ServerConfig struct {
	// GRPCPort is the port the gRPC receiver listens on (default 50051).
	GRPCPort int `yaml:"grpc_port"`

	// HTTPPort is the port the REST API and WebSocket hub listen on (default 8080).
	HTTPPort int `yaml:"http_port"`

	// Auth configures how the server authenticates incoming gRPC and REST clients.
	Auth AuthConfig `yaml:"auth"`

	// Snapshot controls in-memory snapshot retention.
	Snapshot SnapshotConfig `yaml:"snapshot"`

	// Alerts holds rule definitions and webhook delivery targets.
	Alerts AlertsConfig `yaml:"alerts"`
}

// AuthConfig controls client authentication on the server side.
type AuthConfig struct {
	// Mode is one of: apikey | none.
	// "mtls" is supported for future use but requires TLS listener setup.
	Mode string `yaml:"mode"`

	// KeyEnv is the name of the environment variable that holds the expected API key.
	// Used when Mode == "apikey".
	KeyEnv string `yaml:"key_env"`

	// Header is the gRPC metadata key (and HTTP header name) to read the key from.
	// Defaults to "x-api-key" if empty.
	Header string `yaml:"header"`
}

// Key returns the expected API key resolved from the environment.
func (a AuthConfig) Key() string {
	if a.KeyEnv == "" {
		return ""
	}
	return os.Getenv(a.KeyEnv)
}

// EffectiveHeader returns the configured header name, or the default "x-api-key".
func (a AuthConfig) EffectiveHeader() string {
	if a.Header != "" {
		return a.Header
	}
	return "x-api-key"
}

// SnapshotConfig controls in-memory snapshot retention.
type SnapshotConfig struct {
	// TTL is how long a source's snapshot remains in the store after its last update.
	// When TTL elapses without a new snapshot from a source, the entry is evicted.
	// Default: 5m.
	TTL time.Duration `yaml:"ttl"`
}

// Load reads and parses the config file at path, returning the server configuration.
// Missing fields are filled with sensible defaults before validation.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("server config: read %q: %w", path, err)
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("server config: parse yaml: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("server config: %w", err)
	}

	return cfg, nil
}

// defaults returns a Config pre-populated with default values.
func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCPort: DefaultGRPCPort,
			HTTPPort: DefaultHTTPPort,
			Snapshot: SnapshotConfig{
				TTL: DefaultSnapshotTTL,
			},
		},
	}
}

// validate checks structural constraints on the parsed configuration.
func validate(cfg *Config) error {
	if cfg.Server.GRPCPort <= 0 || cfg.Server.GRPCPort > 65535 {
		return fmt.Errorf("server.grpc_port %d is out of range [1, 65535]", cfg.Server.GRPCPort)
	}
	if cfg.Server.HTTPPort <= 0 || cfg.Server.HTTPPort > 65535 {
		return fmt.Errorf("server.http_port %d is out of range [1, 65535]", cfg.Server.HTTPPort)
	}
	switch cfg.Server.Auth.Mode {
	case "apikey", "mtls", "none", "":
	default:
		return fmt.Errorf("server.auth.mode %q unknown: want apikey|mtls|none", cfg.Server.Auth.Mode)
	}
	if cfg.Server.Snapshot.TTL < 0 {
		return fmt.Errorf("server.snapshot.ttl must not be negative")
	}
	return nil
}
