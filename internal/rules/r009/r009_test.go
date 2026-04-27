package r009_test

import (
	"context"
	"testing"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r009"
)

// ── Pure-logic tests (no DB required) ────────────────────────────────────────
// DB-dependent integration tests live in r009_integration_test.go (build tag: integration).
// ConsecutiveV0Count is pre-supplied to bypass the DB query.

func TestCheck_BelowThreshold_Pass(t *testing.T) {
	in := r009.Input{
		UID:                "uid-alice",
		ConsecutiveV0Count: 5,
	}
	res, err := r009.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Warned {
		t.Error("5 < 10: should not warn")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", res.AuditEvent.Verdict.Result)
	}
}

func TestCheck_ExactThreshold_Warn(t *testing.T) {
	in := r009.Input{
		UID:                "uid-bob",
		ConsecutiveV0Count: 10,
	}
	res, err := r009.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Warned {
		t.Error("exactly 10 should trigger warn (condition is >=)")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictFlag {
		t.Errorf("verdict: got %q, want FLAG (warn)", res.AuditEvent.Verdict.Result)
	}
	if res.AuditEvent.Verdict.Severity != "warn" {
		t.Errorf("severity: got %q, want warn", res.AuditEvent.Verdict.Severity)
	}
}

func TestCheck_AboveThreshold_Warn(t *testing.T) {
	in := r009.Input{
		UID:                "uid-charlie",
		ConsecutiveV0Count: 25,
	}
	res, err := r009.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Warned {
		t.Error("25 >= 10: should warn")
	}
	if res.AuditEvent.Evidence.MatchedPattern == nil {
		t.Error("WARN event must include matched_pattern")
	}
	if res.AuditEvent.Verdict.AutoActioned {
		t.Error("WARN event must not be auto_actioned")
	}
	if res.ConsecutiveV0Count != 25 {
		t.Errorf("ConsecutiveV0Count: got %d, want 25", res.ConsecutiveV0Count)
	}
}

func TestCheck_CustomThreshold_Warn(t *testing.T) {
	in := r009.Input{
		UID:                "uid-dave",
		ConsecutiveV0Count: 7,
		Threshold:          5,
	}
	res, err := r009.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Warned {
		t.Error("7 >= custom threshold 5: should warn")
	}
}

func TestCheck_CustomThreshold_Pass(t *testing.T) {
	in := r009.Input{
		UID:                "uid-eve",
		ConsecutiveV0Count: 4,
		Threshold:          5,
	}
	res, err := r009.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Warned {
		t.Error("4 < custom threshold 5: should not warn")
	}
}

func TestCheck_WarnSeverity_PassEvent(t *testing.T) {
	// PASS events from a warn-severity rule must carry severity "warn".
	in := r009.Input{
		UID:                "uid-frank",
		ConsecutiveV0Count: 3,
	}
	res, _ := r009.Check(context.Background(), nil, in)
	if res.AuditEvent.Verdict.Severity != "warn" {
		t.Errorf("PASS from warn rule: severity got %q, want warn", res.AuditEvent.Verdict.Severity)
	}
}

func TestCheck_NoBlock_OnWarn(t *testing.T) {
	// R-009 must never block — only warn.
	in := r009.Input{
		UID:                "uid-grace",
		ConsecutiveV0Count: 100,
	}
	res, _ := r009.Check(context.Background(), nil, in)
	if res.AuditEvent.Verdict.Result == audit.VerdictBlock {
		t.Error("R-009 must never produce a BLOCK verdict")
	}
	if res.AuditEvent.ActionTaken.AppealEligible {
		t.Error("WARN events must not be appeal_eligible")
	}
}

func TestCheck_RuleMetadata(t *testing.T) {
	in := r009.Input{
		UID:                "uid-meta",
		ConsecutiveV0Count: 1,
	}
	res, _ := r009.Check(context.Background(), nil, in)
	if res.AuditEvent.Rule.RuleID != r009.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.AuditEvent.Rule.RuleID, r009.RuleID)
	}
	if res.AuditEvent.Scope.Trigger != r009.Trigger {
		t.Errorf("trigger: got %q, want %q", res.AuditEvent.Scope.Trigger, r009.Trigger)
	}
	if res.AuditEvent.Subject.Type != "user" {
		t.Errorf("subject.type: got %q, want user", res.AuditEvent.Subject.Type)
	}
	if res.AuditEvent.Subject.ID != "uid-meta" {
		t.Errorf("subject.id: got %q, want uid-meta", res.AuditEvent.Subject.ID)
	}
}
