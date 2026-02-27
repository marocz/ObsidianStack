// Package config loads the server-side configuration from the `server:` section
// of config.yaml (the `agent:` key is ignored by the server binary).
//
// Config fields:
//   - GRPCPort     — port for the gRPC receiver (default 50051)
//   - HTTPPort     — port for the REST API and WebSocket hub (default 8080)
//   - Auth.Mode    — "apikey" or "none"; "mtls" supported for future use
//   - Auth.KeyEnv  — environment variable holding the expected API key
//   - Auth.Header  — gRPC metadata/HTTP header name (default "x-api-key")
//   - Snapshot.TTL — how long a source snapshot remains live (default 5m)
//
// Load(path) applies defaults before unmarshalling, then validates.
package config
