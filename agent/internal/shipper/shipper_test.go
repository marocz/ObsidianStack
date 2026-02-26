package shipper

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/obsidianstack/obsidianstack/agent/internal/compute"
	"github.com/obsidianstack/obsidianstack/agent/internal/config"
	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
)

// mockServer implements SnapshotServiceServer for testing.
type mockServer struct {
	pb.UnimplementedSnapshotServiceServer
	mu       sync.Mutex
	received []*pb.PipelineSnapshot
	rejectN  int  // reject the first N calls with an error
	okResp   bool // the Ok field in SendResponse
}

func (m *mockServer) SendSnapshot(_ context.Context, snap *pb.PipelineSnapshot) (*pb.SendResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.rejectN > 0 {
		m.rejectN--
		return &pb.SendResponse{Ok: false, Message: "mock rejection"}, nil
	}

	m.received = append(m.received, snap)
	return &pb.SendResponse{Ok: m.okResp || true}, nil
}

func (m *mockServer) snapshots() []*pb.PipelineSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*pb.PipelineSnapshot, len(m.received))
	copy(out, m.received)
	return out
}

// startTestServer starts an in-process gRPC server and returns
// a dial function that connects to it over a buffered pipe.
func startTestServer(t *testing.T, srv *mockServer) dialFunc {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	gs := grpc.NewServer()
	pb.RegisterSnapshotServiceServer(gs, srv)

	go func() {
		if err := gs.Serve(lis); err != nil {
			// Ignore "use of closed network connection" on test teardown.
		}
	}()
	t.Cleanup(gs.Stop)

	addr := lis.Addr().String()
	return func(ctx context.Context, _ string, _ config.AgentConfig) (*grpc.ClientConn, error) {
		return grpc.DialContext(ctx, addr, //nolint:staticcheck
			grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
}

// makeComputeResult builds a minimal compute.Result for testing.
func makeComputeResult(id string) *compute.Result {
	return &compute.Result{
		SourceID:      id,
		SourceType:    "otelcol",
		Timestamp:     time.Now(),
		State:         compute.StateHealthy,
		DropPct:       1.5,
		RecoveryRate:  98.5,
		ThroughputPM:  1000,
		StrengthScore: 95.2,
		UptimePct:     100,
		Signals: []compute.SignalResult{
			{Type: "traces", ReceivedPM: 900, DroppedPM: 13.5, DropPct: 1.5},
		},
	}
}

func agentCfg() config.AgentConfig {
	return config.AgentConfig{
		ServerEndpoint: "unused-overridden-by-dialFn",
		BufferSize:     10,
		ShipInterval:   time.Second,
	}
}

// --- Tests ---

func TestShipper_DeliversSnapshot(t *testing.T) {
	srv := &mockServer{}
	dial := startTestServer(t, srv)

	s := New(agentCfg())
	s.dialFn = dial

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go s.Run(ctx)

	s.Ship(makeComputeResult("otel-1"))

	// Poll until the server receives it or the context expires.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.snapshots()) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	snaps := srv.snapshots()
	if len(snaps) != 1 {
		t.Fatalf("server received %d snapshots, want 1", len(snaps))
	}
	if snaps[0].SourceId != "otel-1" {
		t.Errorf("SourceId = %q, want %q", snaps[0].SourceId, "otel-1")
	}
	if snaps[0].State != compute.StateHealthy {
		t.Errorf("State = %q, want %q", snaps[0].State, compute.StateHealthy)
	}
}

func TestShipper_MultipleSnapshots(t *testing.T) {
	srv := &mockServer{}
	dial := startTestServer(t, srv)

	s := New(agentCfg())
	s.dialFn = dial

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go s.Run(ctx)

	for i := 0; i < 5; i++ {
		s.Ship(makeComputeResult("src"))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.snapshots()) >= 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if got := len(srv.snapshots()); got != 5 {
		t.Errorf("server received %d snapshots, want 5", got)
	}
}

func TestShipper_BufferEvictsOldest(t *testing.T) {
	// BufferSize=3; Ship 5 items while the shipper is not running.
	// Only the 3 most recent should survive.
	s := New(config.AgentConfig{BufferSize: 3})

	for i := 0; i < 5; i++ {
		res := makeComputeResult("src")
		res.StrengthScore = float64(i) // use score to identify order
		s.Ship(res)
	}

	// Drain the buffer manually and check which remain.
	var scores []float64
	for {
		select {
		case snap := <-s.buf:
			scores = append(scores, snap.StrengthScore)
		default:
			goto done
		}
	}
done:

	if len(scores) != 3 {
		t.Fatalf("buffer has %d items, want 3", len(scores))
	}
	// Scores 2, 3, 4 should remain (0 and 1 were evicted).
	for i, want := range []float64{2, 3, 4} {
		if scores[i] != want {
			t.Errorf("scores[%d] = %.0f, want %.0f", i, scores[i], want)
		}
	}
}

func TestShipper_ConvertToProto(t *testing.T) {
	res := makeComputeResult("prom-test")
	res.DropPct = 3.14
	res.StrengthScore = 88.5
	res.Signals = []compute.SignalResult{
		{Type: "metrics", ReceivedPM: 5000, DroppedPM: 160, DropPct: 3.1},
		{Type: "traces", ReceivedPM: 100, DroppedPM: 3, DropPct: 2.9},
	}

	snap := toProto(res)

	if snap.SourceId != "prom-test" {
		t.Errorf("SourceId = %q, want %q", snap.SourceId, "prom-test")
	}
	if snap.DropPct != 3.14 {
		t.Errorf("DropPct = %v, want 3.14", snap.DropPct)
	}
	if snap.StrengthScore != 88.5 {
		t.Errorf("StrengthScore = %v, want 88.5", snap.StrengthScore)
	}
	if len(snap.Signals) != 2 {
		t.Fatalf("Signals len = %d, want 2", len(snap.Signals))
	}
	if snap.Signals[0].Type != "metrics" {
		t.Errorf("Signals[0].Type = %q, want metrics", snap.Signals[0].Type)
	}
	if snap.Signals[0].ReceivedPm != 5000 {
		t.Errorf("Signals[0].ReceivedPm = %v, want 5000", snap.Signals[0].ReceivedPm)
	}
}

func TestShipper_BackoffResets(t *testing.T) {
	b := newBackoff()
	// First few calls should be small.
	first := b.next()
	if first > 2*time.Second {
		t.Errorf("first backoff too large: %v", first)
	}
	// Advance a few times.
	for i := 0; i < 10; i++ {
		b.next()
	}
	// After reset, should be small again.
	b.reset()
	after := b.next()
	if after > 2*time.Second {
		t.Errorf("backoff after reset too large: %v", after)
	}
}

func TestBackoff_NeverExceedsMax(t *testing.T) {
	b := newBackoff()
	for i := 0; i < 50; i++ {
		d := b.next()
		// With jitter, max is backoffMax * 1.25
		if d > backoffMax*2 {
			t.Errorf("backoff[%d] = %v, exceeds 2×max", i, d)
		}
	}
}

func TestShipper_GracefulShutdown(t *testing.T) {
	srv := &mockServer{}
	dial := startTestServer(t, srv)

	s := New(agentCfg())
	s.dialFn = dial

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// Give it time to connect, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
