// Package ws implements the WebSocket hub for obsidianstack-server.
//
// Hub manages a set of connected clients and broadcasts the current pipeline
// snapshot to all of them on a configurable interval (default 5s in production).
//
// New(store, interval) creates a Hub.
// Hub.Run(ctx) starts the broadcast ticker — blocks until ctx is cancelled,
// then closes all active connections.
// Hub.ServeHTTP upgrades an HTTP connection to WebSocket, sends the current
// snapshot immediately on connect, then streams updates on each tick.
//
// Message format sent to clients:
//
//	{
//	  "event": "snapshot",
//	  "data":  { /* same schema as GET /api/v1/snapshot */ }
//	}
//
// The upgrader accepts all origins. Apply CORS restrictions at the reverse
// proxy level. WebSocket endpoint is mounted at /ws/stream by the server.
package ws
