// Package shipper sends PipelineSnapshot protobuf messages to obsidianstack-server
// via gRPC (SnapshotService.SendSnapshot unary RPC).
//
// Shipper.Ship() is non-blocking: results are converted to proto and placed in
// an in-memory channel (default capacity 1000). When the buffer is full the
// oldest entry is evicted so the latest health data is always preserved.
//
// Shipper.Run() drains the buffer in a loop, reconnecting with truncated
// exponential backoff (1s→60s, ±25% jitter) on connection or send errors.
// Permanent gRPC errors (Unauthenticated, PermissionDenied, InvalidArgument)
// discard the snapshot immediately rather than retrying.
//
// Auth: mTLS via credentials.NewTLS(), API key via gRPC metadata header,
// or insecure (plaintext) for local development.
//
// The dialFn field is injectable for testing (bufconn / net.Listen).
package shipper
