package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
	"github.com/obsidianstack/obsidianstack/server/internal/store"
)

// Handler is the HTTP handler for all /api/v1/* endpoints.
// It reads pipeline state from the snapshot store and returns JSON responses.
type Handler struct {
	store *store.Store
	mux   *http.ServeMux
}

// New creates a Handler wired to the given snapshot store and registers all routes.
func New(st *store.Store) http.Handler {
	h := &Handler{store: st, mux: http.NewServeMux()}

	h.mux.HandleFunc("/api/v1/health", h.health)
	h.mux.HandleFunc("/api/v1/pipelines", h.listPipelines)
	h.mux.HandleFunc("/api/v1/pipelines/", h.getPipeline) // subtree — extracts {id}
	h.mux.HandleFunc("/api/v1/signals", h.signals)
	h.mux.HandleFunc("/api/v1/alerts", h.alerts)
	h.mux.HandleFunc("/api/v1/certs", h.certs)
	h.mux.HandleFunc("/api/v1/snapshot", h.snapshot)

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// --- route handlers ---------------------------------------------------------

// health returns GET /api/v1/health — overall health score and state counts.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	entries := h.store.List()
	resp := HealthResponse{
		PipelineCount: len(entries),
	}

	if len(entries) == 0 {
		resp.State = "unknown"
		jsonResp(w, http.StatusOK, resp)
		return
	}

	var totalScore float64
	for _, e := range entries {
		totalScore += e.Snapshot.StrengthScore
		switch e.Snapshot.State {
		case "healthy":
			resp.HealthyCount++
		case "degraded":
			resp.DegradedCount++
		case "critical":
			resp.CriticalCount++
		default:
			resp.UnknownCount++
		}
	}

	resp.OverallScore = totalScore / float64(len(entries))
	resp.State = stateFromScore(resp.OverallScore)
	jsonResp(w, http.StatusOK, resp)
}

// listPipelines returns GET /api/v1/pipelines — all live pipelines.
func (h *Handler) listPipelines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	entries := h.store.List()
	out := make([]PipelineResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, toPipelineResponse(e))
	}
	jsonResp(w, http.StatusOK, out)
}

// getPipeline returns GET /api/v1/pipelines/{id} — a single live pipeline.
func (h *Handler) getPipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/pipelines/")
	if id == "" {
		// Redirect bare /api/v1/pipelines/ to list handler.
		h.listPipelines(w, r)
		return
	}

	e, ok := h.store.Get(id)
	if !ok {
		jsonErr(w, http.StatusNotFound, "pipeline not found")
		return
	}
	// Exclude stale entries — treat them as not found.
	if time.Since(e.UpdatedAt) > h.store.TTL() {
		jsonErr(w, http.StatusNotFound, "pipeline not found")
		return
	}

	jsonResp(w, http.StatusOK, toPipelineResponse(e))
}

// signals returns GET /api/v1/signals — aggregated metrics/logs/traces across
// all live pipelines.
func (h *Handler) signals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	entries := h.store.List()
	agg := map[string]*struct{ recv, drop float64 }{
		"metrics": {},
		"logs":    {},
		"traces":  {},
	}

	for _, e := range entries {
		for _, sig := range e.Snapshot.Signals {
			if a, ok := agg[sig.Type]; ok {
				a.recv += sig.ReceivedPm
				a.drop += sig.DroppedPm
			}
		}
	}

	resp := SignalsResponse{
		Metrics: toAggregate(agg["metrics"].recv, agg["metrics"].drop),
		Logs:    toAggregate(agg["logs"].recv, agg["logs"].drop),
		Traces:  toAggregate(agg["traces"].recv, agg["traces"].drop),
	}
	jsonResp(w, http.StatusOK, resp)
}

// alerts returns GET /api/v1/alerts — active alerts (empty until T021).
func (h *Handler) alerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	jsonResp(w, http.StatusOK, []struct{}{})
}

// certs returns GET /api/v1/certs — cert status per source (empty until T011).
func (h *Handler) certs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Collect cert info from live snapshots.
	entries := h.store.List()
	type certEntry struct {
		SourceID string `json:"source_id"`
		Endpoint string `json:"endpoint"`
		AuthType string `json:"auth_type"`
		Status   string `json:"status"`
		DaysLeft int32  `json:"days_left"`
		Issuer   string `json:"issuer,omitempty"`
		NotAfter string `json:"not_after,omitempty"`
	}
	out := make([]certEntry, 0)
	for _, e := range entries {
		for _, c := range e.Snapshot.Certs {
			out = append(out, certEntry{
				SourceID: e.Snapshot.SourceId,
				Endpoint: c.Endpoint,
				AuthType: c.AuthType,
				Status:   c.Status,
				DaysLeft: c.DaysLeft,
				Issuer:   c.Issuer,
				NotAfter: c.NotAfter,
			})
		}
	}
	jsonResp(w, http.StatusOK, out)
}

// snapshot returns GET /api/v1/snapshot — full JSON dump of all live pipelines.
func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	entries := h.store.List()
	pipelines := make([]PipelineResponse, 0, len(entries))
	for _, e := range entries {
		pipelines = append(pipelines, toPipelineResponse(e))
	}

	jsonResp(w, http.StatusOK, SnapshotResponse{
		Pipelines:   pipelines,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// --- helpers ----------------------------------------------------------------

func jsonResp(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	jsonResp(w, code, errorResponse{Error: msg})
}

// stateFromScore converts a 0–100 score to a health state string.
// Mirrors the thresholds in agent/internal/compute.
func stateFromScore(score float64) string {
	switch {
	case score >= 85:
		return "healthy"
	case score >= 60:
		return "degraded"
	default:
		return "critical"
	}
}

// toPipelineResponse maps a store.Entry to its JSON representation.
func toPipelineResponse(e *store.Entry) PipelineResponse {
	snap := e.Snapshot
	sigs := make([]SignalResponse, 0, len(snap.Signals))
	for _, s := range snap.Signals {
		sigs = append(sigs, SignalResponse{
			Type:       s.Type,
			ReceivedPM: s.ReceivedPm,
			DroppedPM:  s.DroppedPm,
			DropPct:    s.DropPct,
		})
	}
	return PipelineResponse{
		SourceID:         snap.SourceId,
		SourceType:       snap.SourceType,
		NodeType:         snap.NodeType,
		Cluster:          snap.Cluster,
		Namespace:        snap.Namespace,
		State:            snap.State,
		DropPct:          snap.DropPct,
		RecoveryRate:     snap.RecoveryRate,
		ThroughputPerMin: snap.ThroughputPerMin,
		LatencyP50Ms:     snap.LatencyP50Ms,
		LatencyP95Ms:     snap.LatencyP95Ms,
		LatencyP99Ms:     snap.LatencyP99Ms,
		StrengthScore:    snap.StrengthScore,
		UptimePct:        snap.UptimePct,
		ErrorMessage:     snap.ErrorMessage,
		Signals:          sigs,
		LastSeen:         e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// toAggregate computes a SignalAggregate from raw totals.
func toAggregate(recv, drop float64) SignalAggregate {
	agg := SignalAggregate{ReceivedPM: recv, DroppedPM: drop}
	total := recv + drop
	if total > 0 {
		agg.DropPct = drop / total * 100
	}
	return agg
}

// snapWithSignals is a convenience constructor used internally and in tests.
func snapWithSignals(id string, score float64, state string, sigs []*pb.SignalStats) *pb.PipelineSnapshot {
	return &pb.PipelineSnapshot{
		SourceId:      id,
		SourceType:    "otelcol",
		State:         state,
		StrengthScore: score,
		Signals:       sigs,
	}
}
