// Package store provides the in-memory snapshot store for obsidianstack-server.
//
// Store is a thread-safe map[sourceID]*Entry with TTL-based eviction.
// Each Entry holds the latest PipelineSnapshot received from that source
// and the time it was last updated.
//
// Put(snap) inserts or replaces the entry for snap.SourceId.
// Get(id) returns the entry (may be stale); List() excludes stale entries.
// Evict(now) removes entries older than TTL and returns the count removed.
// Run(ctx) runs a background eviction loop, ticking at TTL/2.
//
// The now field is injectable so tests can control time deterministically.
package store
