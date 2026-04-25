package r005_test

import (
	"context"
	"testing"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r005"
)

// ── Pure-logic tests (no DB required) ────────────────────────────────────────
// DB-dependent integration tests live in r005_integration_test.go (build tag: integration).
// TotalMinutesInWindow is pre-supplied to bypass the DB query.

func TestCheck_BelowCeiling_Pass(t *testing.T) {
	in := r005.Input{
		UID:                  "uid-alice",
		TaskID:               "task-001",
		WindowEndUTC:         time.Now().UTC(),
		TotalMinutesInWindow: 480, // 8 hours — well below 960
	}
	res, err := r005.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", res.Event.Verdict.Result)
	}
	if res.TotalMinutesInWindow != 480 {
		t.Errorf("TotalMinutesInWindow: got %.0f, want 480", res.TotalMinutesInWindow)
	}
}

func TestCheck_ExactCeiling_Pass(t *testing.T) {
	in := r005.Input{
		UID:                  "uid-bob",
		TaskID:               "task-002",
		WindowEndUTC:         time.Now().UTC(),
		TotalMinutesInWindow: 960, // exactly at ceiling — should PASS
	}
	res, err := r005.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictPass {
		t.Errorf("at exactly 960 min should PASS; got %q", res.Event.Verdict.Result)
	}
}

func TestCheck_AboveCeiling_Warn(t *testing.T) {
	in := r005.Input{
		UID:                  "uid-charlie",
		TaskID:               "task-003",
		WindowEndUTC:         time.Now().UTC(),
		TotalMinutesInWindow: 1020, // 17 hours — exceeds 960
	}
	res, err := r005.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictFlag {
		t.Errorf("verdict: got %q, want FLAG (warn)", res.Event.Verdict.Result)
	}
	if res.Event.Verdict.Severity != "warn" {
		t.Errorf("severity: got %q, want warn", res.Event.Verdict.Severity)
	}
	if res.Event.Verdict.AutoActioned {
		t.Error("WARN event must not be auto_actioned")
	}
	if res.Event.Evidence.MatchedPattern == nil {
		t.Error("WARN event must include a matched_pattern")
	}
	if res.TotalMinutesInWindow != 1020 {
		t.Errorf("TotalMinutesInWindow: got %.0f, want 1020", res.TotalMinutesInWindow)
	}
}

func TestCheck_CustomCeiling_Warn(t *testing.T) {
	in := r005.Input{
		UID:                  "uid-dave",
		TaskID:               "task-004",
		WindowEndUTC:         time.Now().UTC(),
		TotalMinutesInWindow: 500,
		MaxMinutes:           480, // lower custom ceiling
	}
	res, err := r005.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictFlag {
		t.Errorf("500 min > custom 480 min ceiling should FLAG; got %q", res.Event.Verdict.Result)
	}
}

func TestCheck_WarnSeverity_PassEvent(t *testing.T) {
	// Even PASS events from a warn-severity rule should carry severity "warn".
	in := r005.Input{
		UID:                  "uid-eve",
		TaskID:               "task-005",
		WindowEndUTC:         time.Now().UTC(),
		TotalMinutesInWindow: 100,
	}
	res, _ := r005.Check(context.Background(), nil, in)
	if res.Event.Verdict.Severity != "warn" {
		t.Errorf("PASS from warn rule: severity got %q, want warn", res.Event.Verdict.Severity)
	}
}

func TestCheck_RuleMetadata(t *testing.T) {
	in := r005.Input{
		UID:                  "uid-test",
		TaskID:               "task-x",
		WindowEndUTC:         time.Now().UTC(),
		TotalMinutesInWindow: 100,
	}
	res, _ := r005.Check(context.Background(), nil, in)
	if res.Event.Rule.RuleID != r005.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.Event.Rule.RuleID, r005.RuleID)
	}
	if res.Event.Scope.Trigger != r005.Trigger {
		t.Errorf("trigger: got %q, want %q", res.Event.Scope.Trigger, r005.Trigger)
	}
}
