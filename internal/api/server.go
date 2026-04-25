// Package api implements the Phase B minimum REST API for LinkRSP.
// Routes follow the contract in docs/spec/openapi-v1.0-draft.md.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server holds shared dependencies for all HTTP handlers.
type Server struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

// RegisterRoutes wires all API routes onto mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tasks", s.createTask)
	mux.HandleFunc("GET /api/v1/tasks/{task_id}", s.getTask)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/attestations", s.createAttestation)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/settlement/preview", s.previewSettlement)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/settlement/commit", s.commitSettlement)
	mux.HandleFunc("GET /api/v1/audit-events", s.listAuditEvents)
	mux.HandleFunc("GET /api/v1/audit-events/{event_id}", s.getAuditEvent)
}

// BlockedResponse is returned (HTTP 409) when a rule blocks an operation.
type BlockedResponse struct {
	Error        string `json:"error"`
	RuleID       string `json:"rule_id"`
	RuleVersion  string `json:"rule_version"`
	Reason       string `json:"reason,omitempty"`
	AuditEventID string `json:"audit_event_id"`
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.Logger.Error("writeJSON encode", "error", err)
	}
}

func (s *Server) errorJSON(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) decodeBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// isNotFound returns true for pgx "no rows" errors.
func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
