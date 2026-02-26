package shipper

import (
	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"

	"github.com/obsidianstack/obsidianstack/agent/internal/compute"
)

// toProto converts a compute.Result into a PipelineSnapshot protobuf message
// ready to be sent over gRPC to obsidianstack-server.
//
// Fields not yet populated by Phase 1 (node_type, cluster, namespace,
// latency percentiles, certs) are left at their zero values and will
// be filled in by later tasks (T011 adds cert data, K8s metadata via labels).
func toProto(r *compute.Result) *pb.PipelineSnapshot {
	snap := &pb.PipelineSnapshot{
		SourceId:        r.SourceID,
		SourceType:      r.SourceType,
		TimestampUnix:   r.Timestamp.Unix(),
		State:           r.State,
		DropPct:         r.DropPct,
		RecoveryRate:    r.RecoveryRate,
		ThroughputPerMin: r.ThroughputPM,
		StrengthScore:   r.StrengthScore,
		UptimePct:       r.UptimePct,
	}

	if r.State == compute.StateUnknown && r.DropPct == 0 {
		// Preserve the unknown signal; server interprets empty counters + unknown
		// state correctly without needing an explicit error_message field.
	}

	for _, sig := range r.Signals {
		snap.Signals = append(snap.Signals, &pb.SignalStats{
			Type:       sig.Type,
			ReceivedPm: sig.ReceivedPM,
			DroppedPm:  sig.DroppedPM,
			DropPct:    sig.DropPct,
		})
	}

	return snap
}
