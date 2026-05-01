//go:build integration

package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPatchSemanticAuditJob_PersistsWritebackAuditEvent(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)

	jobID := mustUUIDString()
	// Seed a job directly into the queue table.
	_, err := server.Pool.Exec(
		t.Context(),
		`INSERT INTO semantic_audit_jobs
			(job_id, rule_id, subject_type, subject_id, text_ref, status, idempotency_key)
		  VALUES
			($1::UUID, 'S-014', 'task', 'task-seed-001', 'seed-text', 'human_review', 'seed-semjob-1')`,
		jobID,
	)
	if err != nil {
		t.Fatalf("seed semantic_audit_jobs: %v", err)
	}

	patchBody := `{"status":"done","verdict":"PASS","confidence":0.7}`
	resp := performJSONRequest(t, mux, http.MethodPatch, "/api/v1/internal/semantic-audit-jobs/"+jobID, patchBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("patch semantic audit job: got %d want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	// The patch should have persisted a job-scoped audit event.
	auditResp := performJSONRequest(t, mux, http.MethodGet, "/api/v1/audit-events?subject_type=semantic_audit_job&subject_id="+jobID+"&rule_id=S-014&limit=10", "")
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit events query: got %d want %d; body=%s", auditResp.Code, http.StatusOK, auditResp.Body.String())
	}
	auditEvents := decodeJSON[auditEventsListResponse](t, auditResp)
	if auditEvents.Total != 1 {
		t.Fatalf("audit events total: got %d want 1; body=%s", auditEvents.Total, auditResp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(auditEvents.Events[0].Payload, &payload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}
	verdict, ok := payload["verdict"].(map[string]any)
	if !ok {
		t.Fatalf("payload.verdict missing or invalid: %v", payload["verdict"])
	}
	if v, _ := verdict["result"].(string); v != "PASS" {
		t.Fatalf("payload.verdict.result: got %v want PASS", verdict["result"])
	}
	if conf, ok := verdict["confidence"].(float64); !ok || conf != 0.7 {
		t.Fatalf("payload.verdict.confidence: got %v want 0.7", verdict["confidence"])
	}
}
