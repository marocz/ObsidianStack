package receiver

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
	"github.com/obsidianstack/obsidianstack/server/internal/store"
)

// Receiver implements pb.SnapshotServiceServer.
// It validates each incoming PipelineSnapshot and stores it in the state store.
type Receiver struct {
	pb.UnimplementedSnapshotServiceServer
	store *store.Store
}

// New creates a Receiver that writes accepted snapshots to st.
func New(st *store.Store) *Receiver {
	return &Receiver{store: st}
}

// SendSnapshot is the unary RPC handler called by obsidianstack-agent instances.
// It validates the snapshot, stores it, and returns a confirmation.
// Authentication is enforced by the gRPC server interceptor before this is called.
func (r *Receiver) SendSnapshot(ctx context.Context, snap *pb.PipelineSnapshot) (*pb.SendResponse, error) {
	if snap.SourceId == "" {
		return nil, status.Error(codes.InvalidArgument, "source_id is required")
	}

	r.store.Put(snap)

	slog.Debug("receiver: snapshot stored",
		"source_id", snap.SourceId,
		"source_type", snap.SourceType,
		"state", snap.State,
		"score", snap.StrengthScore,
	)

	return &pb.SendResponse{Ok: true}, nil
}
