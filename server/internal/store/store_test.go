package store

import (
	"sync"
	"testing"
	"time"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
)

func snap(id string) *pb.PipelineSnapshot {
	return &pb.PipelineSnapshot{SourceId: id, SourceType: "otelcol"}
}

// fixedClock returns a func() time.Time that always returns t.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestPutAndGet(t *testing.T) {
	st := New(5 * time.Minute)
	st.Put(snap("src-1"))

	e, ok := st.Get("src-1")
	if !ok {
		t.Fatal("Get: expected entry, got none")
	}
	if e.Snapshot.SourceId != "src-1" {
		t.Errorf("SourceId: got %q, want src-1", e.Snapshot.SourceId)
	}
}

func TestGet_Missing(t *testing.T) {
	st := New(5 * time.Minute)
	_, ok := st.Get("unknown")
	if ok {
		t.Fatal("Get on empty store: expected false, got true")
	}
}

func TestPut_Overwrites(t *testing.T) {
	st := New(5 * time.Minute)
	s1 := &pb.PipelineSnapshot{SourceId: "src", State: "healthy"}
	s2 := &pb.PipelineSnapshot{SourceId: "src", State: "degraded"}

	st.Put(s1)
	st.Put(s2)

	e, ok := st.Get("src")
	if !ok {
		t.Fatal("Get: expected entry after two Puts")
	}
	if e.Snapshot.State != "degraded" {
		t.Errorf("State: got %q, want degraded", e.Snapshot.State)
	}
}

func TestList_ExcludesStale(t *testing.T) {
	base := time.Now()
	st := New(5 * time.Minute)

	// Put two entries at different times.
	st.now = fixedClock(base.Add(-10 * time.Minute)) // stale
	st.Put(snap("old"))

	st.now = fixedClock(base) // live
	st.Put(snap("new"))

	// List uses current time = base.
	st.now = fixedClock(base)
	entries := st.List()

	if len(entries) != 1 {
		t.Fatalf("List: got %d entries, want 1", len(entries))
	}
	if entries[0].Snapshot.SourceId != "new" {
		t.Errorf("List[0].SourceId: got %q, want new", entries[0].Snapshot.SourceId)
	}
}

func TestCount_IncludesStale(t *testing.T) {
	base := time.Now()
	st := New(5 * time.Minute)

	st.now = fixedClock(base.Add(-10 * time.Minute))
	st.Put(snap("old"))

	st.now = fixedClock(base)
	st.Put(snap("new"))

	// Count includes both; stale not yet evicted.
	if n := st.Count(); n != 2 {
		t.Errorf("Count: got %d, want 2", n)
	}
}

func TestEvict_RemovesStale(t *testing.T) {
	base := time.Now()
	st := New(5 * time.Minute)

	st.now = fixedClock(base.Add(-10 * time.Minute))
	st.Put(snap("old1"))
	st.Put(snap("old2"))

	st.now = fixedClock(base)
	st.Put(snap("live"))

	removed := st.Evict(base)
	if removed != 2 {
		t.Errorf("Evict: removed %d, want 2", removed)
	}
	if st.Count() != 1 {
		t.Errorf("Count after evict: got %d, want 1", st.Count())
	}
}

func TestEvict_NoOp_AllLive(t *testing.T) {
	base := time.Now()
	st := New(5 * time.Minute)

	st.now = fixedClock(base)
	st.Put(snap("src"))

	removed := st.Evict(base)
	if removed != 0 {
		t.Errorf("Evict on live entry: removed %d, want 0", removed)
	}
}

func TestMultipleSources(t *testing.T) {
	st := New(5 * time.Minute)
	ids := []string{"otel", "prom", "loki"}
	for _, id := range ids {
		st.Put(snap(id))
	}

	entries := st.List()
	if len(entries) != 3 {
		t.Errorf("List: got %d entries, want 3", len(entries))
	}
}

func TestConcurrentPuts(t *testing.T) {
	st := New(5 * time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			st.Put(&pb.PipelineSnapshot{SourceId: "concurrent", State: "healthy"})
		}(i)
	}
	wg.Wait()

	// Should have exactly one entry (all same source ID).
	if st.Count() != 1 {
		t.Errorf("Count after concurrent puts: got %d, want 1", st.Count())
	}
}

func TestConcurrentMixedOps(t *testing.T) {
	st := New(5 * time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			st.Put(&pb.PipelineSnapshot{SourceId: "src-a"})
		}()
		go func() {
			defer wg.Done()
			st.List()
		}()
	}
	wg.Wait()
}
