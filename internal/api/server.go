// Package api implements the Phase B minimum REST API for LinkRSP.
// Routes follow the contract in docs/spec/openapi-v1.0-draft.md.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server holds shared dependencies for all HTTP handlers.
type Server struct {
	Pool                      *pgxpool.Pool
	Logger                    *slog.Logger
	Env                       string
	APISharedSecret           string
	R006AccelerationThreshold float64
	R010GenesisEndTime        time.Time
}

func (s *Server) RegisterHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api/v1/healthz", s.healthz)
}

// RegisterRoutes wires all API routes onto mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tasks", s.requireSharedSecret(s.createTask))
	mux.HandleFunc("GET /api/v1/tasks/{task_id}", s.getTask)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/attestations", s.requireSharedSecret(s.createAttestation))
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/settlement/preview", s.requireSharedSecret(s.previewSettlement))
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/settlement/commit", s.requireSharedSecret(s.commitSettlement))
	mux.HandleFunc("GET /api/v1/audit-events", s.requireSharedSecret(s.listAuditEvents))
	mux.HandleFunc("GET /api/v1/audit-events/{event_id}", s.requireSharedSecret(s.getAuditEvent))
	mux.HandleFunc("GET /api/v1/internal/lrs-ledger-query", s.requireSharedSecret(s.lrsLedgerQuery))
	mux.HandleFunc("GET /api/v1/internal/attestation-index-query", s.requireSharedSecret(s.attestationIndexQuery))
	mux.HandleFunc("GET /api/v1/internal/semantic-audit-jobs", s.requireSharedSecret(s.listSemanticAuditJobs))
	mux.HandleFunc("GET /api/v1/internal/semantic-audit-jobs/stats", s.requireSharedSecret(s.semanticAuditJobStats))
	mux.HandleFunc("POST /api/v1/internal/semantic-audit-jobs/enqueue", s.requireSharedSecret(s.enqueueSemanticAuditJob))
	mux.HandleFunc("GET /api/v1/internal/semantic-audit-jobs/{job_id}/replay", s.requireSharedSecret(s.semanticAuditJobReplay))
	mux.HandleFunc("PATCH /api/v1/internal/semantic-audit-jobs/{job_id}", s.requireSharedSecret(s.patchSemanticAuditJob))
}

func (s *Server) requireSharedSecret(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.APISharedSecret == "" {
			next(w, r)
			return
		}

		providedSecret := r.Header.Get("X-API-Shared-Secret")
		if subtle.ConstantTimeCompare([]byte(providedSecret), []byte(s.APISharedSecret)) != 1 {
			s.errorJSON(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
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
