// Package config loads and watches the agent configuration file (config.yaml).
//
// Top-level types:
//   - Config{Agent, Server} — full config tree parsed from YAML
//   - AgentConfig — server_endpoint, scrape_interval, ship_interval, buffer_size,
//     sources [], server_auth
//   - Source — id, type (otelcol|prometheus|loki|jaeger|http), endpoint, auth, tls
//   - AuthConfig — mode (mtls|apikey|bearer|none), cert/key/ca files, header,
//     key_env, token_env; Key() and Token() resolve from environment variables
//   - ServerConfig, ServerAuthConfig, AlertsConfig, StorageConfig — server-side
//     settings parsed but used by the server binary, not the agent
//
// Load(path) reads the YAML file, applies defaults (30s scrape, 15s ship,
// 1000 buffer, ports 50051/8080), then validates required fields and enums.
//
// Watch(ctx, path, onChange) uses fsnotify to detect file changes and calls
// onChange with the newly parsed Config. It handles the rename→create pattern
// used by atomic-save editors (vim, VS Code) by re-adding the watch after
// a rename event.
package config
