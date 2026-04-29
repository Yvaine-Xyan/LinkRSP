//go:build integration

package api

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
)

type createdTaskResponse struct {
	TaskID          string `json:"task_id"`
	UIDSubmitter    string `json:"uid_submitter"`
	DescriptionText string `json:"description_text"`
	StartTimeUTC    string `json:"start_time_utc"`
	EndTimeUTC      string `json:"end_time_utc"`
	CreatedAtUTC    string `json:"created_at_utc"`
}

type createdAttestationResponse struct {
	AttestationID     string `json:"attestation_id"`
	TaskID            string `json:"task_id"`
	VerificationLevel int    `json:"verification_level"`
	TimestampUTC      string `json:"timestamp_utc"`
	CreatedAtUTC      string `json:"created_at_utc"`
}

type auditEventsListResponse struct {
	Events []auditEventItem `json:"events"`
	Total  int              `json:"total"`
}

func TestSettlementCommit_PersistsLedgerAndAuditEvents(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)

	createTaskBody := `{
		"uid_submitter":"uid-int-001",
		"community_id":"11111111-1111-1111-1111-111111111111",
		"description_text":"integration task",
		"start_time_utc":"2026-04-27T08:00:00Z",
		"end_time_utc":"2026-04-27T10:00:00Z",
		"location_hash":"loc-hash-001"
	}`
	createTaskResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks", createTaskBody)
	if createTaskResp.Code != http.StatusCreated {
		t.Fatalf("create task: got %d want %d; body=%s", createTaskResp.Code, http.StatusCreated, createTaskResp.Body.String())
	}
	task := decodeJSON[createdTaskResponse](t, createTaskResp)

	createAttestationBody := `{
		"verification_level":1,
		"timestamp_utc":"2026-04-27T10:05:00Z",
		"location_hash":"loc-hash-001"
	}`
	createAttestationResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks/"+task.TaskID+"/attestations", createAttestationBody)
	if createAttestationResp.Code != http.StatusCreated {
		t.Fatalf("create attestation: got %d want %d; body=%s", createAttestationResp.Code, http.StatusCreated, createAttestationResp.Body.String())
	}
	_ = decodeJSON[createdAttestationResponse](t, createAttestationResp)

	previewResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks/"+task.TaskID+"/settlement/preview", "")
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview settlement: got %d want %d; body=%s", previewResp.Code, http.StatusOK, previewResp.Body.String())
	}
	preview := decodeJSON[settlementResponse](t, previewResp)
	if preview.LedgerEntryID != nil {
		t.Fatal("preview settlement must not return ledger_entry_id")
	}
	if !slices.Equal(preview.RulesChecked, []string{"R-001", "R-005", "R-006", "R-010"}) {
		t.Fatalf("preview rules_checked: got %v", preview.RulesChecked)
	}
	if preview.Formula.DBase != 1.0 {
		t.Fatalf("DBase: got %.2f want 1.0", preview.Formula.DBase)
	}
	if preview.Formula.KGlobal != 1.0 {
		t.Fatalf("KGlobal: got %.2f want 1.0", preview.Formula.KGlobal)
	}
	if preview.Formula.CreditsDelta != 60 {
		t.Fatalf("CreditsDelta: got %.2f want 60", preview.Formula.CreditsDelta)
	}

	commitResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks/"+task.TaskID+"/settlement/commit", "")
	if commitResp.Code != http.StatusOK {
		t.Fatalf("commit settlement: got %d want %d; body=%s", commitResp.Code, http.StatusOK, commitResp.Body.String())
	}
	commit := decodeJSON[settlementResponse](t, commitResp)
	if commit.LedgerEntryID == nil || *commit.LedgerEntryID == "" {
		t.Fatal("commit settlement must return ledger_entry_id")
	}
	if !slices.Equal(commit.RulesChecked, []string{"R-001", "R-005", "R-006", "R-010"}) {
		t.Fatalf("commit rules_checked: got %v", commit.RulesChecked)
	}

	commitAgainResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks/"+task.TaskID+"/settlement/commit", "")
	if commitAgainResp.Code != http.StatusOK {
		t.Fatalf("commit settlement again: got %d want %d; body=%s", commitAgainResp.Code, http.StatusOK, commitAgainResp.Body.String())
	}
	commitAgain := decodeJSON[settlementResponse](t, commitAgainResp)
	if commitAgain.LedgerEntryID == nil || *commitAgain.LedgerEntryID != *commit.LedgerEntryID {
		t.Fatalf("idempotent commit should return same ledger entry id: first=%v second=%v", commit.LedgerEntryID, commitAgain.LedgerEntryID)
	}

	windowStart := url.QueryEscape("2026-04-27T00:00:00Z")
	windowEnd := url.QueryEscape("2026-04-28T00:00:00Z")
	ledgerResp := performJSONRequest(t, mux, http.MethodGet, "/api/v1/internal/lrs-ledger-query?uid=uid-int-001&window_start_utc="+windowStart+"&window_end_utc="+windowEnd, "")
	if ledgerResp.Code != http.StatusOK {
		t.Fatalf("ledger query: got %d want %d; body=%s", ledgerResp.Code, http.StatusOK, ledgerResp.Body.String())
	}
	ledger := decodeJSON[lrsLedgerQueryResponse](t, ledgerResp)
	if len(ledger.Entries) != 1 {
		t.Fatalf("ledger entries: got %d want 1", len(ledger.Entries))
	}
	if ledger.Entries[0].TaskID != task.TaskID {
		t.Fatalf("ledger task_id: got %q want %q", ledger.Entries[0].TaskID, task.TaskID)
	}
	if ledger.EntryCount != 1 {
		t.Fatalf("ledger entry_count: got %d want 1", ledger.EntryCount)
	}
	if ledger.ProjectedBalance != 60 {
		t.Fatalf("ledger projected_balance: got %.2f want 60", ledger.ProjectedBalance)
	}
	if ledger.Entries[0].ProjectedBalance != 60 {
		t.Fatalf("ledger entry projected_balance: got %.2f want 60", ledger.Entries[0].ProjectedBalance)
	}

	attestationResp := performJSONRequest(t, mux, http.MethodGet, "/api/v1/internal/attestation-index-query?uid=uid-int-001&window_start_utc="+windowStart+"&window_end_utc="+windowEnd, "")
	if attestationResp.Code != http.StatusOK {
		t.Fatalf("attestation index query: got %d want %d; body=%s", attestationResp.Code, http.StatusOK, attestationResp.Body.String())
	}
	attestations := decodeJSON[attestationIndexQueryResponse](t, attestationResp)
	if len(attestations.Items) != 1 {
		t.Fatalf("attestation items: got %d want 1", len(attestations.Items))
	}
	if attestations.Items[0].TaskID != task.TaskID {
		t.Fatalf("attestation task_id: got %q want %q", attestations.Items[0].TaskID, task.TaskID)
	}
	if attestations.Items[0].VerificationLevel != 1 {
		t.Fatalf("verification_level: got %d want 1", attestations.Items[0].VerificationLevel)
	}

	auditResp := performJSONRequest(t, mux, http.MethodGet, "/api/v1/audit-events?subject_type=task&subject_id="+task.TaskID, "")
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit events query: got %d want %d; body=%s", auditResp.Code, http.StatusOK, auditResp.Body.String())
	}
	auditEvents := decodeJSON[auditEventsListResponse](t, auditResp)
	if auditEvents.Total != 6 {
		t.Fatalf("audit events total: got %d want 6", auditEvents.Total)
	}

	ruleSeen := map[string]int{}
	for _, event := range auditEvents.Events {
		ruleSeen[event.RuleID]++
	}
	for _, ruleID := range []string{"R-001", "R-005", "R-006", "R-010"} {
		if ruleSeen[ruleID] != 1 {
			t.Fatalf("expected exactly one audit event for %s, got counts=%v", ruleID, ruleSeen)
		}
	}
	if ruleSeen["R-004"] != 1 || ruleSeen["R-003"] != 1 {
		t.Fatalf("expected task creation audit events once each, got counts=%v", ruleSeen)
	}
}

func TestCreateAttestation_PersistsR009AuditEvent(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)

	for index := 0; index < 10; index++ {
		createTaskBody := `{
			"uid_submitter":"uid-r009-001",
			"description_text":"v0 task",
			"start_time_utc":"2026-04-27T08:00:00Z",
			"end_time_utc":"2026-04-27T09:00:00Z"
		}`
		createTaskResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks", createTaskBody)
		if createTaskResp.Code != http.StatusCreated {
			t.Fatalf("create task %d: got %d want %d; body=%s", index, createTaskResp.Code, http.StatusCreated, createTaskResp.Body.String())
		}
		task := decodeJSON[createdTaskResponse](t, createTaskResp)

		createAttestationBody := `{
			"verification_level":0,
			"timestamp_utc":"2026-04-27T10:05:00Z"
		}`
		createAttestationResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks/"+task.TaskID+"/attestations", createAttestationBody)
		if createAttestationResp.Code != http.StatusCreated {
			t.Fatalf("create attestation %d: got %d want %d; body=%s", index, createAttestationResp.Code, http.StatusCreated, createAttestationResp.Body.String())
		}
	}

	auditResp := performJSONRequest(t, mux, http.MethodGet, "/api/v1/audit-events?subject_type=user&subject_id=uid-r009-001&rule_id=R-009&limit=20", "")
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit events query: got %d want %d; body=%s", auditResp.Code, http.StatusOK, auditResp.Body.String())
	}
	auditEvents := decodeJSON[auditEventsListResponse](t, auditResp)
	if auditEvents.Total < 1 {
		t.Fatalf("r009 audit events total: got %d want >= 1", auditEvents.Total)
	}
	if auditEvents.Events[0].RuleID != "R-009" {
		t.Fatalf("expected latest rule_id R-009, got %q", auditEvents.Events[0].RuleID)
	}
}

func TestInternalQueries_ValidateRFC3339Window(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)
	resp := performJSONRequest(t, mux, http.MethodGet, "/api/v1/internal/lrs-ledger-query?uid=uid-x&window_start_utc=bad&window_end_utc=2026-04-28T00:00:00Z", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status code: got %d want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
}

func TestSettlementPreview_UsesUTCStrings(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)

	createTaskBody := `{
		"uid_submitter":"uid-int-utc",
		"description_text":"utc task",
		"start_time_utc":"2026-04-27T08:00:00+08:00",
		"end_time_utc":"2026-04-27T10:00:00+08:00"
	}`
	createTaskResp := performJSONRequest(t, mux, http.MethodPost, "/api/v1/tasks", createTaskBody)
	if createTaskResp.Code != http.StatusCreated {
		t.Fatalf("create task: got %d want %d; body=%s", createTaskResp.Code, http.StatusCreated, createTaskResp.Body.String())
	}
	task := decodeJSON[createdTaskResponse](t, createTaskResp)

	start := mustParseRFC3339(t, task.StartTimeUTC)
	end := mustParseRFC3339(t, task.EndTimeUTC)
	if start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatal("task timestamps must be normalized to UTC")
	}
}
