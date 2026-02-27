package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/obsidianstack/obsidianstack/server/internal/api"
	"github.com/obsidianstack/obsidianstack/server/internal/store"
)

const (
	// writeTimeout is the deadline for a single write to a client.
	writeTimeout = 10 * time.Second

	// pongWait is how long to wait for a pong response before treating the
	// connection as dead.
	pongWait = 60 * time.Second

	// pingPeriod controls how often the server sends WebSocket ping frames.
	// Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// sendBufSize is the per-client outgoing message buffer depth.
	sendBufSize = 16
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Allow all origins — callers should apply CORS at the reverse-proxy level.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Message is the JSON envelope sent to clients on every broadcast tick.
type Message struct {
	Event string           `json:"event"`
	Data  api.SnapshotResponse `json:"data"`
}

// Hub manages WebSocket client connections and broadcasts the current pipeline
// snapshot to all connected clients every interval.
type Hub struct {
	store    *store.Store
	interval time.Duration

	mu      sync.RWMutex
	clients map[*client]struct{}
}

// client represents one connected WebSocket client.
type client struct {
	conn *websocket.Conn
	send chan []byte
}

// New creates a Hub that reads from st and broadcasts every interval.
func New(st *store.Store, interval time.Duration) *Hub {
	return &Hub{
		store:    st,
		interval: interval,
		clients:  make(map[*client]struct{}),
	}
}

// Run starts the broadcast ticker loop. It sends the current snapshot to all
// connected clients every interval. Run blocks until ctx is cancelled, then
// closes all active connections.
func (h *Hub) Run(ctx context.Context) {
	t := time.NewTicker(h.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case <-t.C:
			h.broadcast()
		}
	}
}

// ServeHTTP upgrades the HTTP connection to WebSocket and serves the client.
// It sends the current snapshot immediately on connect, then continues to
// receive broadcasts from the ticker loop. Blocks until the connection closes.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// upgrader has already written the error response.
		return
	}

	c := &client{
		conn: conn,
		send: make(chan []byte, sendBufSize),
	}
	h.register(c)
	defer h.unregister(c)

	// Send the current snapshot immediately so the UI has data right away.
	if data, err := h.buildMessage(); err == nil {
		select {
		case c.send <- data:
		default:
		}
	}

	go c.writePump()
	c.readPump() // blocks until connection closes
}

// Count returns the number of currently connected clients.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// --- internal ---------------------------------------------------------------

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *Hub) broadcast() {
	data, err := h.buildMessage()
	if err != nil {
		return
	}

	h.mu.RLock()
	targets := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		select {
		case c.send <- data:
		default:
			// Client's outgoing buffer is full — disconnect it.
			h.unregister(c)
		}
	}
}

func (h *Hub) buildMessage() ([]byte, error) {
	msg := Message{
		Event: "snapshot",
		Data:  api.BuildSnapshot(h.store),
	}
	return json.Marshal(msg)
}

func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		close(c.send)
		delete(h.clients, c)
	}
}

// writePump drains the client's send channel and forwards messages to the
// WebSocket connection. It also sends periodic ping frames. Runs in its own
// goroutine per client.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				// Channel was closed (hub is shutting down or client removed).
				c.conn.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads frames from the connection to process control messages (pong,
// close) and detect disconnects. Blocks until the connection closes.
func (c *client) readPump() {
	defer c.conn.Close()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}
