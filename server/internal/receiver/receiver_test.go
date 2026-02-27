package receiver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
	"github.com/obsidianstack/obsidianstack/server/internal/auth"
	"github.com/obsidianstack/obsidianstack/server/internal/receiver"
	"github.com/obsidianstack/obsidianstack/server/internal/store"
)

// startServer starts a gRPC server with the given interceptor and returns a
// connected client and a cleanup function. Uses a random TCP port.
func startServer(t *testing.T, interceptor grpc.UnaryServerInterceptor) (pb.SnapshotServiceClient, *store.Store) {
	t.Helper()

	st := store.New(5 * time.Minute)
	rec := receiver.New(st)

	srv := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	pb.RegisterSnapshotServiceServer(srv, rec)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go srv.Serve(lis) //nolint:errcheck

	t.Cleanup(func() {
		srv.Stop()
		lis.Close()
	})

	conn, err := grpc.Dial(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	) //nolint:staticcheck
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return pb.NewSnapshotServiceClient(conn), st
}

// allowAll is a no-op interceptor that passes every call through.
func allowAll(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return handler(ctx, req)
}

func TestSendSnapshot_StoresSnapshot(t *testing.T) {
	client, st := startServer(t, allowAll)

	snap := &pb.PipelineSnapshot{
		SourceId:      "otel-prod",
		SourceType:    "otelcol",
		State:         "healthy",
		StrengthScore: 92.5,
	}

	resp, err := client.SendSnapshot(context.Background(), snap)
	if err != nil {
		t.Fatalf("SendSnapshot: %v", err)
	}
	if !resp.Ok {
		t.Errorf("Ok: got false, want true")
	}

	e, ok := st.Get("otel-prod")
	if !ok {
		t.Fatal("store.Get: expected entry, got none")
	}
	if e.Snapshot.State != "healthy" {
		t.Errorf("State: got %q, want healthy", e.Snapshot.State)
	}
	if e.Snapshot.StrengthScore != 92.5 {
		t.Errorf("StrengthScore: got %v, want 92.5", e.Snapshot.StrengthScore)
	}
}

func TestSendSnapshot_MissingSourceId_InvalidArgument(t *testing.T) {
	client, _ := startServer(t, allowAll)

	_, err := client.SendSnapshot(context.Background(), &pb.PipelineSnapshot{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code: got %v, want InvalidArgument", code)
	}
}

func TestSendSnapshot_MultipleSnapshots_AllStored(t *testing.T) {
	client, st := startServer(t, allowAll)

	sources := []string{"otel", "prometheus", "loki"}
	for _, id := range sources {
		_, err := client.SendSnapshot(context.Background(), &pb.PipelineSnapshot{
			SourceId:   id,
			SourceType: id,
			State:      "healthy",
		})
		if err != nil {
			t.Fatalf("SendSnapshot %q: %v", id, err)
		}
	}

	if n := st.Count(); n != 3 {
		t.Errorf("store.Count: got %d, want 3", n)
	}
	for _, id := range sources {
		if _, ok := st.Get(id); !ok {
			t.Errorf("store.Get(%q): not found", id)
		}
	}
}

func TestSendSnapshot_UpdateExistingSource(t *testing.T) {
	client, st := startServer(t, allowAll)

	ctx := context.Background()
	_, err := client.SendSnapshot(ctx, &pb.PipelineSnapshot{SourceId: "src", State: "healthy"})
	if err != nil {
		t.Fatalf("first SendSnapshot: %v", err)
	}
	_, err = client.SendSnapshot(ctx, &pb.PipelineSnapshot{SourceId: "src", State: "degraded"})
	if err != nil {
		t.Fatalf("second SendSnapshot: %v", err)
	}

	if st.Count() != 1 {
		t.Errorf("store.Count: got %d, want 1 (updates, not appends)", st.Count())
	}
	e, _ := st.Get("src")
	if e.Snapshot.State != "degraded" {
		t.Errorf("State: got %q, want degraded", e.Snapshot.State)
	}
}

func TestSendSnapshot_WithAPIKeyInterceptor_CorrectKey_Passes(t *testing.T) {
	i := auth.APIKeyInterceptor("apikey", "x-api-key", "testkey")
	client, st := startServer(t, i)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-api-key", "testkey")
	_, err := client.SendSnapshot(ctx, &pb.PipelineSnapshot{SourceId: "src", State: "healthy"})
	if err != nil {
		t.Fatalf("SendSnapshot with correct key: %v", err)
	}
	if st.Count() != 1 {
		t.Errorf("store.Count: got %d, want 1", st.Count())
	}
}

func TestSendSnapshot_WithAPIKeyInterceptor_WrongKey_Rejected(t *testing.T) {
	i := auth.APIKeyInterceptor("apikey", "x-api-key", "testkey")
	client, _ := startServer(t, i)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-api-key", "wrongkey")
	_, err := client.SendSnapshot(ctx, &pb.PipelineSnapshot{SourceId: "src", State: "healthy"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("code: got %v, want Unauthenticated", code)
	}
}

func TestSendSnapshot_WithAPIKeyInterceptor_MissingKey_Rejected(t *testing.T) {
	i := auth.APIKeyInterceptor("apikey", "x-api-key", "testkey")
	client, _ := startServer(t, i)

	// No key in metadata.
	_, err := client.SendSnapshot(context.Background(), &pb.PipelineSnapshot{SourceId: "src"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("code: got %v, want Unauthenticated", code)
	}
}
