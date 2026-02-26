// Package receiver implements the gRPC server that accepts PipelineSnapshot
// streams from obsidianstack-agent instances. It validates agent authentication
// (mTLS client cert CN or X-API-Key header) and forwards snapshots to the store.
package receiver
