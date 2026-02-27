package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
	"github.com/obsidianstack/obsidianstack/server/internal/api"
	"github.com/obsidianstack/obsidianstack/server/internal/store"
)

// --- test helpers -----------------------------------------------------------

func newStore(snaps ...*pb.PipelineSnapshot) *store.Store {
	st := store.New(5 * time.Minute)
	for _, s := range snaps {
		st.Put(s)
	}
	return st
}

func snap(id, state string, score float64) *pb.PipelineSnapshot {
	return &pb.PipelineSnapshot{
		SourceId:      id,
		SourceType:    "otelcol",
		State:         state,
		StrengthScore: score,
		DropPct:       2.5,
		RecoveryRate:  97.0,
		UptimePct:     100.0,
	}
}

func snapWithSigs(id string, sigs []*pb.SignalStats) *pb.PipelineSnapshot {
	return &pb.PipelineSnapshot{
		SourceId:      id,
		SourceType:    "prometheus",
		State:         "healthy",
		StrengthScore: 90.0,
		Signals:       sigs,
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr
}

func decode(t *testing.T, rr *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v (body: %s)", err, rr.Body.String())
	}
}

// --- /api/v1/health ---------------------------------------------------------

func TestHealth_EmptyStore(t *testing.T) {
	h := api.New(newStore())
	rr := get(t, h, "/api/v1/health")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp map[string]interface{}
	decode(t, rr, &resp)

	if resp["state"] != "unknown" {
		t.Errorf("state: got %v, want unknown", resp["state"])
	}
	if resp["pipeline_count"].(float64) != 0 {
		t.Errorf("pipeline_count: got %v, want 0", resp["pipeline_count"])
	}
}

func TestHealth_HealthyPipeline(t *testing.T) {
	h := api.New(newStore(snap("otel", "healthy", 92.0)))
	rr := get(t, h, "/api/v1/health")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp map[string]interface{}
	decode(t, rr, &resp)

	if resp["state"] != "healthy" {
		t.Errorf("state: got %v, want healthy", resp["state"])
	}
	if resp["overall_score"].(float64) != 92.0 {
		t.Errorf("overall_score: got %v, want 92.0", resp["overall_score"])
	}
	if resp["healthy_count"].(float64) != 1 {
		t.Errorf("healthy_count: got %v, want 1", resp["healthy_count"])
	}
	if resp["pipeline_count"].(float64) != 1 {
		t.Errorf("pipeline_count: got %v, want 1", resp["pipeline_count"])
	}
}

func TestHealth_MixedStates(t *testing.T) {
	h := api.New(newStore(
		snap("a", "healthy", 90.0),
		snap("b", "degraded", 70.0),
		snap("c", "critical", 40.0),
	))
	rr := get(t, h, "/api/v1/health")
	var resp map[string]interface{}
	decode(t, rr, &resp)

	if resp["healthy_count"].(float64) != 1 {
		t.Errorf("healthy_count: got %v, want 1", resp["healthy_count"])
	}
	if resp["degraded_count"].(float64) != 1 {
		t.Errorf("degraded_count: got %v, want 1", resp["degraded_count"])
	}
	if resp["critical_count"].(float64) != 1 {
		t.Errorf("critical_count: got %v, want 1", resp["critical_count"])
	}
	// overall = avg(90+70+40)/3 = 66.67 → degraded
	if resp["state"] != "degraded" {
		t.Errorf("state: got %v, want degraded", resp["state"])
	}
}

func TestHealth_MethodNotAllowed(t *testing.T) {
	h := api.New(newStore())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rr.Code)
	}
}

// --- /api/v1/pipelines ------------------------------------------------------

func TestListPipelines_Empty(t *testing.T) {
	h := api.New(newStore())
	rr := get(t, h, "/api/v1/pipelines")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp []interface{}
	decode(t, rr, &resp)
	if len(resp) != 0 {
		t.Errorf("pipelines: got %d items, want 0", len(resp))
	}
}

func TestListPipelines_Multiple(t *testing.T) {
	h := api.New(newStore(
		snap("otel", "healthy", 92.0),
		snap("prom", "degraded", 70.0),
		snap("loki", "critical", 40.0),
	))
	rr := get(t, h, "/api/v1/pipelines")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp []interface{}
	decode(t, rr, &resp)
	if len(resp) != 3 {
		t.Errorf("pipelines: got %d, want 3", len(resp))
	}
}

func TestListPipelines_FieldsPresent(t *testing.T) {
	h := api.New(newStore(snap("otel", "healthy", 92.5)))
	rr := get(t, h, "/api/v1/pipelines")
	var resp []map[string]interface{}
	decode(t, rr, &resp)

	if len(resp) != 1 {
		t.Fatalf("got %d items, want 1", len(resp))
	}
	p := resp[0]
	if p["source_id"] != "otel" {
		t.Errorf("source_id: got %v", p["source_id"])
	}
	if p["strength_score"].(float64) != 92.5 {
		t.Errorf("strength_score: got %v, want 92.5", p["strength_score"])
	}
	if p["last_seen"] == "" || p["last_seen"] == nil {
		t.Error("last_seen: missing")
	}
}

func TestListPipelines_MethodNotAllowed(t *testing.T) {
	h := api.New(newStore())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/pipelines", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rr.Code)
	}
}

// --- /api/v1/pipelines/{id} -------------------------------------------------

func TestGetPipeline_Found(t *testing.T) {
	h := api.New(newStore(snap("otel-prod", "healthy", 88.0)))
	rr := get(t, h, "/api/v1/pipelines/otel-prod")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	var p map[string]interface{}
	decode(t, rr, &p)
	if p["source_id"] != "otel-prod" {
		t.Errorf("source_id: got %v", p["source_id"])
	}
	if p["strength_score"].(float64) != 88.0 {
		t.Errorf("strength_score: got %v", p["strength_score"])
	}
}

func TestGetPipeline_NotFound(t *testing.T) {
	h := api.New(newStore())
	rr := get(t, h, "/api/v1/pipelines/does-not-exist")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
}

func TestGetPipeline_MethodNotAllowed(t *testing.T) {
	h := api.New(newStore(snap("src", "healthy", 90.0)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/pipelines/src", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rr.Code)
	}
}

// --- /api/v1/signals --------------------------------------------------------

func TestSignals_NoData(t *testing.T) {
	h := api.New(newStore())
	rr := get(t, h, "/api/v1/signals")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp map[string]map[string]float64
	decode(t, rr, &resp)

	for _, sigType := range []string{"metrics", "logs", "traces"} {
		if resp[sigType]["received_pm"] != 0 {
			t.Errorf("%s.received_pm: got %v, want 0", sigType, resp[sigType]["received_pm"])
		}
	}
}

func TestSignals_Aggregation(t *testing.T) {
	h := api.New(newStore(
		snapWithSigs("a", []*pb.SignalStats{
			{Type: "metrics", ReceivedPm: 1000, DroppedPm: 50},
			{Type: "logs", ReceivedPm: 500, DroppedPm: 10},
		}),
		snapWithSigs("b", []*pb.SignalStats{
			{Type: "metrics", ReceivedPm: 2000, DroppedPm: 100},
			{Type: "traces", ReceivedPm: 300, DroppedPm: 0},
		}),
	))
	rr := get(t, h, "/api/v1/signals")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp map[string]map[string]float64
	decode(t, rr, &resp)

	// metrics: 1000+2000=3000 received, 50+100=150 dropped
	if resp["metrics"]["received_pm"] != 3000 {
		t.Errorf("metrics.received_pm: got %v, want 3000", resp["metrics"]["received_pm"])
	}
	if resp["metrics"]["dropped_pm"] != 150 {
		t.Errorf("metrics.dropped_pm: got %v, want 150", resp["metrics"]["dropped_pm"])
	}
	// drop_pct = 150/(3000+150)*100 ≈ 4.76
	if resp["metrics"]["drop_pct"] <= 0 {
		t.Errorf("metrics.drop_pct: expected > 0, got %v", resp["metrics"]["drop_pct"])
	}
	// logs: 500 received, 10 dropped
	if resp["logs"]["received_pm"] != 500 {
		t.Errorf("logs.received_pm: got %v, want 500", resp["logs"]["received_pm"])
	}
	// traces: 300 received, 0 dropped
	if resp["traces"]["received_pm"] != 300 {
		t.Errorf("traces.received_pm: got %v, want 300", resp["traces"]["received_pm"])
	}
	if resp["traces"]["drop_pct"] != 0 {
		t.Errorf("traces.drop_pct: got %v, want 0", resp["traces"]["drop_pct"])
	}
}

func TestSignals_MethodNotAllowed(t *testing.T) {
	h := api.New(newStore())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/signals", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rr.Code)
	}
}

// --- /api/v1/alerts ---------------------------------------------------------

func TestAlerts_ReturnsEmptyArray(t *testing.T) {
	h := api.New(newStore())
	rr := get(t, h, "/api/v1/alerts")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp []interface{}
	decode(t, rr, &resp)
	if resp == nil {
		t.Error("alerts: got null, want []")
	}
	if len(resp) != 0 {
		t.Errorf("alerts: got %d items, want 0", len(resp))
	}
}

// --- /api/v1/certs ----------------------------------------------------------

func TestCerts_ReturnsEmptyArray_NoCerts(t *testing.T) {
	h := api.New(newStore(snap("otel", "healthy", 90.0))) // snap has no certs
	rr := get(t, h, "/api/v1/certs")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp []interface{}
	decode(t, rr, &resp)
	if len(resp) != 0 {
		t.Errorf("certs: got %d items, want 0", len(resp))
	}
}

func TestCerts_ReturnsCertData(t *testing.T) {
	s := &pb.PipelineSnapshot{
		SourceId: "otel",
		Certs: []*pb.CertStatus{
			{Endpoint: "https://otel:4317", AuthType: "mtls", Status: "valid", DaysLeft: 45},
		},
	}
	h := api.New(newStore(s))
	rr := get(t, h, "/api/v1/certs")

	var resp []map[string]interface{}
	decode(t, rr, &resp)
	if len(resp) != 1 {
		t.Fatalf("certs: got %d, want 1", len(resp))
	}
	if resp[0]["endpoint"] != "https://otel:4317" {
		t.Errorf("endpoint: got %v", resp[0]["endpoint"])
	}
	if resp[0]["status"] != "valid" {
		t.Errorf("status: got %v", resp[0]["status"])
	}
}

// --- /api/v1/snapshot -------------------------------------------------------

func TestSnapshot_Empty(t *testing.T) {
	h := api.New(newStore())
	rr := get(t, h, "/api/v1/snapshot")

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var resp map[string]interface{}
	decode(t, rr, &resp)
	if resp["generated_at"] == "" || resp["generated_at"] == nil {
		t.Error("generated_at: missing")
	}
	pipelines := resp["pipelines"].([]interface{})
	if len(pipelines) != 0 {
		t.Errorf("pipelines: got %d, want 0", len(pipelines))
	}
}

func TestSnapshot_AllLivePipelines(t *testing.T) {
	h := api.New(newStore(
		snap("otel", "healthy", 90.0),
		snap("prom", "degraded", 70.0),
	))
	rr := get(t, h, "/api/v1/snapshot")

	var resp map[string]interface{}
	decode(t, rr, &resp)
	pipelines := resp["pipelines"].([]interface{})
	if len(pipelines) != 2 {
		t.Errorf("pipelines: got %d, want 2", len(pipelines))
	}
}

func TestSnapshot_MethodNotAllowed(t *testing.T) {
	h := api.New(newStore())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/api/v1/snapshot", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rr.Code)
	}
}

// --- Content-Type -----------------------------------------------------------

func TestContentTypeJSON(t *testing.T) {
	h := api.New(newStore())
	for _, path := range []string{
		"/api/v1/health",
		"/api/v1/pipelines",
		"/api/v1/signals",
		"/api/v1/alerts",
		"/api/v1/certs",
		"/api/v1/snapshot",
	} {
		rr := get(t, h, path)
		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("%s Content-Type: got %q, want application/json", path, ct)
		}
	}
}
