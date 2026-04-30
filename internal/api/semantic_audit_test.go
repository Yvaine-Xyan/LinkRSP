package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// noopServer returns a Server with no pool — safe for validation-only paths.
func noopServer() *Server {
	return &Server{
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		APISharedSecret: "",
	}
}

func TestListSemanticAuditJobs_InvalidStatus(t *testing.T) {
	s := noopServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/internal/semantic-audit-jobs?status=bad", nil)
	w := httptest.NewRecorder()
	s.listSemanticAuditJobs(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListSemanticAuditJobs_InvalidLimit(t *testing.T) {
	s := noopServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/internal/semantic-audit-jobs?limit=999", nil)
	w := httptest.NewRecorder()
	s.listSemanticAuditJobs(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPatchSemanticAuditJob_InvalidUUID(t *testing.T) {
	s := noopServer()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/internal/semantic-audit-jobs/not-a-uuid",
		strings.NewReader(`{"status":"done"}`))
	req.SetPathValue("job_id", "not-a-uuid")
	w := httptest.NewRecorder()
	s.patchSemanticAuditJob(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid UUID, got %d", w.Code)
	}
}

func TestPatchSemanticAuditJob_EmptyBody(t *testing.T) {
	s := noopServer()
	req := httptest.NewRequest(http.MethodPatch, "/",
		strings.NewReader(`{}`))
	req.SetPathValue("job_id", "11111111-1111-1111-1111-111111111111")
	w := httptest.NewRecorder()
	s.patchSemanticAuditJob(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty patch, got %d", w.Code)
	}
}

func TestPatchSemanticAuditJob_InvalidStatus(t *testing.T) {
	s := noopServer()
	req := httptest.NewRequest(http.MethodPatch, "/",
		strings.NewReader(`{"status":"invalid"}`))
	req.SetPathValue("job_id", "11111111-1111-1111-1111-111111111111")
	w := httptest.NewRecorder()
	s.patchSemanticAuditJob(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d", w.Code)
	}
}

func TestPatchSemanticAuditJob_InvalidVerdict(t *testing.T) {
	s := noopServer()
	req := httptest.NewRequest(http.MethodPatch, "/",
		strings.NewReader(`{"verdict":"MAYBE"}`))
	req.SetPathValue("job_id", "11111111-1111-1111-1111-111111111111")
	w := httptest.NewRecorder()
	s.patchSemanticAuditJob(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid verdict, got %d", w.Code)
	}
}

func TestPatchSemanticAuditJob_ConfidenceOutOfRange(t *testing.T) {
	s := noopServer()
	req := httptest.NewRequest(http.MethodPatch, "/",
		strings.NewReader(`{"confidence":1.5}`))
	req.SetPathValue("job_id", "11111111-1111-1111-1111-111111111111")
	w := httptest.NewRecorder()
	s.patchSemanticAuditJob(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for confidence > 1, got %d", w.Code)
	}
}
