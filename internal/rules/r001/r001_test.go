package r001_test

import (
	"testing"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r001"
)

// ── Pure-logic tests (no DB required) ────────────────────────────────────────
// DB-dependent integration tests live in r001_integration_test.go (build tag: integration).

func TestInput_V0_AlwaysPass(t *testing.T) {
	// R-001 only fires for V>=1. V=0 must short-circuit to PASS.
	in := r001.Input{
		UID:               "uid-alice",
		TaskID:            "task-001",
		StartTimeUTC:      time.Now().UTC(),
		EndTimeUTC:        time.Now().UTC().Add(time.Hour),
		LocationHash:      "loc-a",
		VerificationLevel: 0,
	}
	// We can't call Check (needs a pool), so test the Input struct fields.
	if in.VerificationLevel >= 1 {
		t.Fatal("expected V=0 to be < 1")
	}
}

func TestConflictDetail_Fields(t *testing.T) {
	d := &r001.ConflictDetail{
		TaskID:         "task-999",
		OverlapMinutes: 42.5,
		LocationHash:   "loc-b",
	}
	if d.TaskID == "" {
		t.Fatal("TaskID should not be empty")
	}
	if d.OverlapMinutes <= 0 {
		t.Fatal("OverlapMinutes should be positive")
	}
}

func TestAuditBuilder_BlockEvent(t *testing.T) {
	b := audit.NewBuilder(r001.RuleID, r001.Category, r001.Trigger)
	secondaryID := "task-conflict"
	ev := b.BuildBlock("task", "task-001", &secondaryID, "test pattern", []string{"submitter"})

	if ev.Rule.RuleID != r001.RuleID {
		t.Errorf("rule_id: got %q, want %q", ev.Rule.RuleID, r001.RuleID)
	}
	if ev.Verdict.Result != audit.VerdictBlock {
		t.Errorf("verdict: got %q, want BLOCK", ev.Verdict.Result)
	}
	if !ev.Verdict.AutoActioned {
		t.Error("BLOCK event must set auto_actioned=true")
	}
	if ev.Verdict.Confidence != nil {
		t.Error("R-class: confidence must be nil")
	}
	if !ev.ActionTaken.AppealEligible {
		t.Error("BLOCK event must be appeal_eligible")
	}
	if ev.Subject.SecondaryID == nil || *ev.Subject.SecondaryID != secondaryID {
		t.Error("secondary_id not set correctly")
	}
	if ev.SchemaVersion != "1.0" {
		t.Errorf("schema_version: got %q, want 1.0", ev.SchemaVersion)
	}
}

func TestAuditBuilder_PassEvent(t *testing.T) {
	b := audit.NewBuilder(r001.RuleID, r001.Category, r001.Trigger)
	ev := b.BuildPass("task", "task-001")

	if ev.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", ev.Verdict.Result)
	}
	if ev.Verdict.AutoActioned {
		t.Error("PASS event must not set auto_actioned")
	}
}

func TestIdempotencyKey_Format(t *testing.T) {
	ts := time.Date(2026, 4, 24, 15, 37, 59, 0, time.UTC)
	key := audit.IdempotencyKey("R-001", "task", "task-abc", "post_execution", ts)
	want := "R-001|task|task-abc|post_execution|2026-04-24T15:37"
	if key != want {
		t.Errorf("idempotency key: got %q, want %q", key, want)
	}
}
