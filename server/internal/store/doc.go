// Package store manages the in-memory pipeline state and optional SQLite
// persistence. It provides a thread-safe snapshot store with TTL eviction
// and time-range query support for historical data.
package store
