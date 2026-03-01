package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Default values applied when fields are absent from the config file.
const (
	DefaultScrapeInterval = 30 * time.Second
	DefaultShipInterval   = 15 * time.Second
	DefaultBufferSize     = 1000
	DefaultGRPCPort       = 50051
	DefaultHTTPPort       = 8080
)

// Config is the top-level configuration for both agent and server.
// Fields map 1:1 to config.example.yaml.
type Config struct {
	Agent  AgentConfig  `yaml:"agent"`
	Server ServerConfig `yaml:"server"`
}

// AgentConfig holds all agent-side settings.
type AgentConfig struct {
	// ServerEndpoint is the gRPC address of obsidianstack-server (host:port).
	ServerEndpoint string `yaml:"server_endpoint"`

	// ScrapeInterval controls how often each source is polled.
	ScrapeInterval time.Duration `yaml:"scrape_interval"`

	// ShipInterval controls how often buffered snapshots are sent to the server.
	ShipInterval time.Duration `yaml:"ship_interval"`

	// BufferSize is the maximum number of snapshots held in memory when
	// the server is unreachable.
	BufferSize int `yaml:"buffer_size"`

	// Sources is the list of pipeline components to monitor.
	Sources []Source `yaml:"sources"`

	// ServerAuth configures how the agent authenticates to obsidianstack-server.
	// Supports the same modes as source auth: mtls | apikey | none.
	ServerAuth AuthConfig `yaml:"server_auth"`
}

// Source describes one monitored pipeline component.
type Source struct {
	// ID is a unique, human-readable identifier for this source.
	ID string `yaml:"id"`

	// Type is the component type: otelcol | prometheus | loki | jaeger | http.
	Type string `yaml:"type"`

	// Endpoint is the full URL of the component's metrics or health endpoint.
	Endpoint string `yaml:"endpoint"`

	// Auth configures how the agent authenticates to this source.
	Auth AuthConfig `yaml:"auth"`

	// TLS holds optional TLS dial options.
	TLS TLSConfig `yaml:"tls"`
}

// AuthConfig specifies the authentication mode for a source.
type AuthConfig struct {
	// Mode is one of: mtls | apikey | bearer | basic | none.
	Mode string `yaml:"mode"`

	// mTLS fields — used when Mode == "mtls".
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"`

	// API key fields — used when Mode == "apikey".
	// Header is the HTTP header name to send the key in.
	Header string `yaml:"header"`
	// KeyEnv is the name of the environment variable that holds the key value.
	KeyEnv string `yaml:"key_env"`

	// Bearer token fields — used when Mode == "bearer".
	// TokenEnv is the name of the environment variable that holds the token.
	TokenEnv string `yaml:"token_env"`

	// Basic auth fields — used when Mode == "basic".
	// Username is the literal username (safe to store in config).
	Username string `yaml:"username"`
	// PasswordEnv is the name of the environment variable that holds the password.
	PasswordEnv string `yaml:"password_env"`
}

// Key returns the API key value resolved from the environment.
// Returns empty string if KeyEnv is unset or the variable is not found.
func (a AuthConfig) Key() string {
	if a.KeyEnv == "" {
		return ""
	}
	return os.Getenv(a.KeyEnv)
}

// Token returns the bearer token value resolved from the environment.
func (a AuthConfig) Token() string {
	if a.TokenEnv == "" {
		return ""
	}
	return os.Getenv(a.TokenEnv)
}

// Password returns the basic-auth password resolved from the environment.
func (a AuthConfig) Password() string {
	if a.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(a.PasswordEnv)
}

// TLSConfig holds per-source TLS dial options.
type TLSConfig struct {
	// InsecureSkipVerify disables TLS certificate verification.
	// Only use this for internal CAs in development environments.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// ServerConfig holds all server-side settings.
type ServerConfig struct {
	// GRPCPort is the port the gRPC receiver listens on.
	GRPCPort int `yaml:"grpc_port"`

	// HTTPPort is the port the REST API and WebSocket hub listen on.
	HTTPPort int `yaml:"http_port"`

	// Auth configures how the server authenticates incoming REST API requests.
	Auth ServerAuthConfig `yaml:"auth"`

	// Alerts holds alerting rule and webhook delivery configuration.
	Alerts AlertsConfig `yaml:"alerts"`

	// Storage configures the optional historical data backend.
	Storage StorageConfig `yaml:"storage"`
}

// ServerAuthConfig configures REST API authentication.
type ServerAuthConfig struct {
	// Mode is one of: apikey | mtls | none.
	Mode string `yaml:"mode"`

	// KeyEnv is the name of the environment variable holding the expected API key.
	KeyEnv string `yaml:"key_env"`
}

// Key returns the server API key resolved from the environment.
func (a ServerAuthConfig) Key() string {
	if a.KeyEnv == "" {
		return ""
	}
	return os.Getenv(a.KeyEnv)
}

// AlertsConfig holds all alerting rules and webhook targets.
type AlertsConfig struct {
	Rules    []AlertRule     `yaml:"rules"`
	Webhooks []WebhookConfig `yaml:"webhooks"`
}

// AlertRule defines a threshold-based alert condition.
type AlertRule struct {
	// Name is the human-readable alert identifier.
	Name string `yaml:"name"`

	// Condition is an expression like "drop_pct > 10" or "cert_days_left < 14".
	Condition string `yaml:"condition"`

	// Severity is one of: critical | warning | info.
	Severity string `yaml:"severity"`

	// Cooldown suppresses re-fires for this duration after an alert fires.
	Cooldown time.Duration `yaml:"cooldown"`
}

// WebhookConfig defines one webhook delivery target.
type WebhookConfig struct {
	// Type is one of: teams | slack | pagerduty | http.
	Type string `yaml:"type"`

	// URLEnv is the name of the environment variable holding the webhook URL.
	URLEnv string `yaml:"url_env"`
}

// URL returns the webhook URL resolved from the environment.
func (w WebhookConfig) URL() string {
	if w.URLEnv == "" {
		return ""
	}
	return os.Getenv(w.URLEnv)
}

// StorageConfig configures the historical data persistence backend.
type StorageConfig struct {
	// Backend selects the storage implementation: sqlite.
	Backend string `yaml:"backend"`

	// Path is the filesystem path for the SQLite database file.
	Path string `yaml:"path"`

	// Retention is how long historical snapshots are kept before deletion.
	Retention time.Duration `yaml:"retention"`
}

// Load reads and parses the YAML config file at path.
// Missing optional fields are filled with sensible defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file: %w", err)
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return cfg, nil
}

// defaults returns a Config pre-populated with default values.
func defaults() *Config {
	return &Config{
		Agent: AgentConfig{
			ScrapeInterval: DefaultScrapeInterval,
			ShipInterval:   DefaultShipInterval,
			BufferSize:     DefaultBufferSize,
		},
		Server: ServerConfig{
			GRPCPort: DefaultGRPCPort,
			HTTPPort: DefaultHTTPPort,
		},
	}
}

// validate checks required fields and structural constraints.
func validate(cfg *Config) error {
	if cfg.Agent.ServerEndpoint == "" {
		return fmt.Errorf("agent.server_endpoint is required")
	}
	if cfg.Agent.ScrapeInterval <= 0 {
		return fmt.Errorf("agent.scrape_interval must be positive")
	}
	if cfg.Agent.ShipInterval <= 0 {
		return fmt.Errorf("agent.ship_interval must be positive")
	}
	if cfg.Agent.BufferSize <= 0 {
		return fmt.Errorf("agent.buffer_size must be positive")
	}
	for i, src := range cfg.Agent.Sources {
		if src.ID == "" {
			return fmt.Errorf("sources[%d]: id is required", i)
		}
		if src.Endpoint == "" {
			return fmt.Errorf("sources[%d] %q: endpoint is required", i, src.ID)
		}
		switch src.Type {
		case "otelcol", "prometheus", "loki", "fluentbit", "jaeger", "http":
		default:
			return fmt.Errorf("sources[%d] %q: unknown type %q", i, src.ID, src.Type)
		}
		switch src.Auth.Mode {
		case "mtls", "apikey", "bearer", "basic", "none", "":
		default:
			return fmt.Errorf("sources[%d] %q: unknown auth mode %q", i, src.ID, src.Auth.Mode)
		}
	}
	return nil
}
