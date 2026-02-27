package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
	"github.com/obsidianstack/obsidianstack/server/internal/store"
	wsHub "github.com/obsidianstack/obsidianstack/server/internal/ws"
)

const testInterval = 20 * time.Millisecond

// --- helpers ----------------------------------------------------------------

func newStore(snaps ...*pb.PipelineSnapshot) *store.Store {
	st := store.New(5 * time.Minute)
	for _, s := range snaps {
		st.Put(s)
	}
	return st
}

func snap(id, state string) *pb.PipelineSnapshot {
	return &pb.PipelineSnapshot{
		SourceId:      id,
		SourceType:    "otelcol",
		State:         state,
		StrengthScore: 90.0,
	}
}

// startHub starts a test HTTP server with the hub as its handler.
// The hub's Run loop is started with a cancellable context.
// Returns the ws:// URL, the hub, and a cleanup function.
func startHub(t *testing.T, st *store.Store) (wsURL string, hub *wsHub.Hub, cancel func()) {
	t.Helper()

	hub = wsHub.New(st, testInterval)
	ctx, cancelFn := context.WithCancel(context.Background())

	srv := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	go hub.Run(ctx)

	t.Cleanup(func() {
		cancelFn()
		srv.Close()
	})

	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	return wsURL, hub, cancelFn
}

// dial connects a WebSocket client to wsURL and returns the connection.
func dial(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", wsURL, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readMessage reads one text message from conn with a short deadline.
func readMessage(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	return msg
}

// --- tests ------------------------------------------------------------------

func TestHub_Connect_ReceivesImmediateSnapshot(t *testing.T) {
	st := newStore(snap("otel", "healthy"))
	wsURL, _, _ := startHub(t, st)

	conn := dial(t, wsURL)
	msg := readMessage(t, conn)

	var m map[string]interface{}
	if err := json.Unmarshal(msg, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["event"] != "snapshot" {
		t.Errorf("event: got %v, want snapshot", m["event"])
	}
	data, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data: missing or wrong type")
	}
	if data["generated_at"] == nil || data["generated_at"] == "" {
		t.Error("generated_at: missing")
	}
}

func TestHub_MessageContainsPipelines(t *testing.T) {
	st := newStore(snap("otel", "healthy"), snap("prom", "degraded"))
	wsURL, _, _ := startHub(t, st)

	conn := dial(t, wsURL)
	msg := readMessage(t, conn)

	var m map[string]interface{}
	json.Unmarshal(msg, &m) //nolint:errcheck
	data := m["data"].(map[string]interface{})
	pipelines, ok := data["pipelines"].([]interface{})
	if !ok {
		t.Fatal("pipelines: missing or wrong type")
	}
	if len(pipelines) != 2 {
		t.Errorf("pipelines: got %d, want 2", len(pipelines))
	}
}

func TestHub_EmptyStore_EmptyPipelines(t *testing.T) {
	wsURL, _, _ := startHub(t, newStore())
	conn := dial(t, wsURL)
	msg := readMessage(t, conn)

	var m map[string]interface{}
	json.Unmarshal(msg, &m) //nolint:errcheck
	data := m["data"].(map[string]interface{})
	pipelines := data["pipelines"].([]interface{})
	if len(pipelines) != 0 {
		t.Errorf("pipelines: got %d, want 0", len(pipelines))
	}
}

func TestHub_CountClients_SingleClient(t *testing.T) {
	wsURL, hub, _ := startHub(t, newStore())

	conn := dial(t, wsURL)
	readMessage(t, conn) // consume initial message

	// Give the hub a moment to register the client.
	time.Sleep(10 * time.Millisecond)
	if n := hub.Count(); n != 1 {
		t.Errorf("Count: got %d, want 1", n)
	}
}

func TestHub_CountClients_MultipleClients(t *testing.T) {
	wsURL, hub, _ := startHub(t, newStore())

	for i := 0; i < 3; i++ {
		conn := dial(t, wsURL)
		readMessage(t, conn) // consume initial message
	}

	time.Sleep(10 * time.Millisecond)
	if n := hub.Count(); n != 3 {
		t.Errorf("Count: got %d, want 3", n)
	}
}

func TestHub_CountClients_DecreasesOnDisconnect(t *testing.T) {
	wsURL, hub, _ := startHub(t, newStore())

	conn := dial(t, wsURL)
	readMessage(t, conn)
	time.Sleep(10 * time.Millisecond)

	if n := hub.Count(); n != 1 {
		t.Errorf("Count before disconnect: got %d, want 1", n)
	}

	conn.Close()
	time.Sleep(50 * time.Millisecond) // let readPump detect the close

	if n := hub.Count(); n != 0 {
		t.Errorf("Count after disconnect: got %d, want 0", n)
	}
}

func TestHub_ReceivesBroadcastOnTick(t *testing.T) {
	st := newStore()
	wsURL, _, _ := startHub(t, st)

	conn := dial(t, wsURL)
	readMessage(t, conn) // consume immediate snapshot (empty store)

	// Add a pipeline after connect.
	st.Put(snap("new-source", "healthy"))

	// The next tick should broadcast a message with the new pipeline.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("waiting for tick broadcast: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(msg, &m) //nolint:errcheck
	data := m["data"].(map[string]interface{})
	pipelines := data["pipelines"].([]interface{})
	if len(pipelines) != 1 {
		t.Errorf("tick broadcast: got %d pipelines, want 1", len(pipelines))
	}
	p := pipelines[0].(map[string]interface{})
	if p["source_id"] != "new-source" {
		t.Errorf("source_id: got %v, want new-source", p["source_id"])
	}
}

func TestHub_AllClientsReceiveBroadcast(t *testing.T) {
	wsURL, _, _ := startHub(t, newStore(snap("src", "healthy")))

	conns := make([]*websocket.Conn, 3)
	for i := 0; i < 3; i++ {
		conns[i] = dial(t, wsURL)
	}

	// All three should receive the initial snapshot.
	for i, conn := range conns {
		msg := readMessage(t, conn)
		var m map[string]interface{}
		if err := json.Unmarshal(msg, &m); err != nil {
			t.Errorf("client %d: unmarshal: %v", i, err)
			continue
		}
		if m["event"] != "snapshot" {
			t.Errorf("client %d: event: got %v, want snapshot", i, m["event"])
		}
	}
}

func TestHub_CancelContextClosesConnections(t *testing.T) {
	wsURL, hub, cancel := startHub(t, newStore())

	conn := dial(t, wsURL)
	readMessage(t, conn)
	time.Sleep(10 * time.Millisecond)

	cancel() // signal shutdown

	// After cancel, hub should close all clients.
	time.Sleep(50 * time.Millisecond)
	if n := hub.Count(); n != 0 {
		t.Errorf("Count after cancel: got %d, want 0", n)
	}
}

func TestHub_NonWebSocketRequest_Returns400(t *testing.T) {
	hub := wsHub.New(newStore(), testInterval)
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer srv.Close()

	// Plain HTTP GET without WebSocket upgrade headers → 400
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
