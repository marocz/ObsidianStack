// Package receiver implements pb.SnapshotServiceServer — the gRPC endpoint
// that accepts PipelineSnapshot messages from obsidianstack-agent instances.
//
// Receiver.SendSnapshot validates that source_id is non-empty
// (codes.InvalidArgument if missing), then calls store.Put to record the
// snapshot. Authentication is enforced upstream by the gRPC server interceptor
// (see package auth), so the receiver itself only performs structural validation.
//
// New(st) wires the receiver to the given snapshot store.
package receiver
