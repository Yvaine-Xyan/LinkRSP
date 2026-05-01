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

func TestEnqueueSemanticAuditJob_AndStats(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)

	enqueueBody := `{
		"rule_id":"S-014",
		"subject_type":"task",
		"subject_id":"task-seed-002",
		"text":"this is a seed text",
		"trigger_words":["seed","foo"]
	}`
	resp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/internal/semantic-audit-jobs/enqueue", enqueueBody)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("enqueue: got %d want %d; body=%s", resp.Code, http.StatusAccepted, resp.Body.String())
	}

	stats := performJSONRequest(t, mux, http.MethodGet, "/api/v1/internal/semantic-audit-jobs/stats", "")
	if stats.Code != http.StatusOK {
		t.Fatalf("stats: got %d want %d; body=%s", stats.Code, http.StatusOK, stats.Body.String())
	}
	decoded := decodeJSON[semanticAuditJobStatsResponse](t, stats)
	if decoded.CountsByStatus["pending"] != 1 {
		t.Fatalf("pending count: got %d want 1; all=%v", decoded.CountsByStatus["pending"], decoded.CountsByStatus)
	}
}

func TestSemanticAuditJobReplay_TaskSubjectBundle(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)

	// Create a real task + attestation + settlement so replay can pull linked evidence.
	createTaskBody := `{
		"uid_submitter":"uid-replay-001",
		"description_text":"replay task",
		"start_time_utc":"2026-04-27T08:00:00Z",
		"end_time_utc":"2026-04-27T08:30:00Z"
	}`
	createTaskResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks", createTaskBody)
	if createTaskResp.Code != http.StatusCreated {
		t.Fatalf("create task: got %d want %d; body=%s", createTaskResp.Code, http.StatusCreated, createTaskResp.Body.String())
	}
	task := decodeJSON[createdTaskResponse](t, createTaskResp)

	createAttestationBody := `{
		"verification_level":1,
		"timestamp_utc":"2026-04-27T08:31:00Z",
		"location_hash":"loc-hash-replay"
	}`
	attResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks/"+task.TaskID+"/attestations", createAttestationBody)
	if attResp.Code != http.StatusCreated {
		t.Fatalf("create attestation: got %d want %d; body=%s", attResp.Code, http.StatusCreated, attResp.Body.String())
	}

	commitResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks/"+task.TaskID+"/settlement/commit", "")
	if commitResp.Code != http.StatusOK {
		t.Fatalf("commit settlement: got %d want %d; body=%s", commitResp.Code, http.StatusOK, commitResp.Body.String())
	}

	enqueueBody := `{
		"rule_id":"S-014",
		"subject_type":"task",
		"subject_id":"` + task.TaskID + `",
		"text":"replay seed text",
		"trigger_words":["replay"]
	}`
	enqueue := performJSONRequest(t, mux, http.MethodPost, "/api/v1/internal/semantic-audit-jobs/enqueue", enqueueBody)
	if enqueue.Code != http.StatusAccepted {
		t.Fatalf("enqueue: got %d want %d; body=%s", enqueue.Code, http.StatusAccepted, enqueue.Body.String())
	}

	list := performJSONRequest(t, mux, http.MethodGet, "/api/v1/internal/semantic-audit-jobs?status=pending&limit=5", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list: got %d want %d; body=%s", list.Code, http.StatusOK, list.Body.String())
	}
	jobs := decodeJSON[listSemanticAuditJobsResponse](t, list)
	if jobs.Total != 1 || len(jobs.Jobs) != 1 {
		t.Fatalf("expected 1 job, got total=%d len=%d", jobs.Total, len(jobs.Jobs))
	}
	jobID := jobs.Jobs[0].JobID

	// Write-back to generate a writeback audit event for replay bundle.
	patch := performJSONRequest(t, mux, http.MethodPatch, "/api/v1/internal/semantic-audit-jobs/"+jobID, `{"status":"done","verdict":"PASS","confidence":0.7}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: got %d want %d; body=%s", patch.Code, http.StatusOK, patch.Body.String())
	}

	replay := performJSONRequest(t, mux, http.MethodGet, "/api/v1/internal/semantic-audit-jobs/"+jobID+"/replay", "")
	if replay.Code != http.StatusOK {
		t.Fatalf("replay: got %d want %d; body=%s", replay.Code, http.StatusOK, replay.Body.String())
	}
	var bundle map[string]any
	_ = json.Unmarshal(replay.Body.Bytes(), &bundle)
	if bundle["job"] == nil || bundle["writeback_audit_events"] == nil {
		t.Fatalf("replay bundle missing required fields: %v", bundle)
	}
}
