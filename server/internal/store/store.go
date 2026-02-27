package store

import (
	"context"
	"log/slog"
	"sync"
	"time"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
)

// Entry is a snapshot together with the time it was last received.
type Entry struct {
	Snapshot  *pb.PipelineSnapshot
	UpdatedAt time.Time
}

// Store is a thread-safe in-memory snapshot store, keyed by source_id.
// A background goroutine (Run) periodically evicts entries that have not
// been updated within the configured TTL.
type Store struct {
	mu   sync.RWMutex
	data map[string]*Entry
	ttl  time.Duration
	now  func() time.Time // injectable for deterministic tests
}

// New creates a Store with the given TTL.
func New(ttl time.Duration) *Store {
	return &Store{
		data: make(map[string]*Entry),
		ttl:  ttl,
		now:  time.Now,
	}
}

// Put stores or replaces the snapshot for snap.SourceId.
// Callers must not modify snap after calling Put.
func (s *Store) Put(snap *pb.PipelineSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[snap.SourceId] = &Entry{
		Snapshot:  snap,
		UpdatedAt: s.now(),
	}
}

// Get returns the Entry for the given source ID and a boolean indicating
// whether an entry was found. The entry may be stale if TTL has elapsed.
func (s *Store) Get(sourceID string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[sourceID]
	return e, ok
}

// List returns a snapshot of all entries whose UpdatedAt is within the TTL.
// Stale entries that have not yet been evicted are excluded.
func (s *Store) List() []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cutoff := s.now().Add(-s.ttl)
	out := make([]*Entry, 0, len(s.data))
	for _, e := range s.data {
		if e.UpdatedAt.After(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

// Count returns the total number of entries currently held, including stale ones.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Evict removes entries whose UpdatedAt is older than now minus TTL.
// It returns the number of entries removed.
func (s *Store) Evict(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := now.Add(-s.ttl)
	removed := 0
	for id, e := range s.data {
		if !e.UpdatedAt.After(cutoff) {
			delete(s.data, id)
			removed++
		}
	}
	return removed
}

// Run starts the background TTL eviction loop. It ticks at half the TTL interval
// (minimum 1 second) so entries are evicted promptly. Run blocks until ctx is
// cancelled.
func (s *Store) Run(ctx context.Context) {
	interval := s.ttl / 2
	if interval < time.Second {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if n := s.Evict(now); n > 0 {
				slog.Debug("store: evicted stale snapshots", "count", n)
			}
		}
	}
}
