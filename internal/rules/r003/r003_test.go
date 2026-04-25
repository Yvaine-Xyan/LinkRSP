package r003_test

import (
	"context"
	"testing"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r003"
)

// ── Pure-logic tests (no DB required) ────────────────────────────────────────

func TestCheck_NormalDuration_Pass(t *testing.T) {
	now := time.Now().UTC()
	in := r003.Input{
		TaskID:       "task-001",
		StartTimeUTC: now,
		EndTimeUTC:   now.Add(2 * time.Hour),
	}
	res, err := r003.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", res.Event.Verdict.Result)
	}
	if res.ComputedMinutes != 120 {
		t.Errorf("ComputedMinutes: got %.1f, want 120", res.ComputedMinutes)
	}
}

func TestCheck_ExactCeiling_Pass(t *testing.T) {
	now := time.Now().UTC()
	in := r003.Input{
		TaskID:       "task-002",
		StartTimeUTC: now,
		EndTimeUTC:   now.Add(1440 * time.Minute),
	}
	res, err := r003.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictPass {
		t.Errorf("at exactly 1440 min should still PASS; got %q", res.Event.Verdict.Result)
	}
}

func TestCheck_OverCeiling_Block(t *testing.T) {
	now := time.Now().UTC()
	in := r003.Input{
		TaskID:       "task-003",
		StartTimeUTC: now,
		EndTimeUTC:   now.Add(1441 * time.Minute),
	}
	res, err := r003.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictBlock {
		t.Errorf("verdict: got %q, want BLOCK", res.Event.Verdict.Result)
	}
	if !res.Event.Verdict.AutoActioned {
		t.Error("BLOCK event must set auto_actioned=true")
	}
}

func TestCheck_DeclaredMinutes_Block(t *testing.T) {
	now := time.Now().UTC()
	in := r003.Input{
		TaskID:          "task-004",
		StartTimeUTC:    now,
		EndTimeUTC:      now.Add(2 * time.Hour),    // computed = 120 min (OK)
		DeclaredMinutes: 1500,                        // declared exceeds ceiling
	}
	res, err := r003.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictBlock {
		t.Errorf("declared excess should BLOCK; got %q", res.Event.Verdict.Result)
	}
}

func TestCheck_CustomCeiling(t *testing.T) {
	now := time.Now().UTC()
	in := r003.Input{
		TaskID:       "task-005",
		StartTimeUTC: now,
		EndTimeUTC:   now.Add(4 * time.Hour), // 240 min
		MaxMinutes:   180,                     // custom ceiling
	}
	res, err := r003.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictBlock {
		t.Errorf("240 min > custom 180 min ceiling should BLOCK; got %q", res.Event.Verdict.Result)
	}
}

func TestCheck_RuleID(t *testing.T) {
	now := time.Now().UTC()
	in := r003.Input{
		TaskID:       "task-x",
		StartTimeUTC: now,
		EndTimeUTC:   now.Add(time.Hour),
	}
	res, _ := r003.Check(context.Background(), nil, in)
	if res.Event.Rule.RuleID != r003.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.Event.Rule.RuleID, r003.RuleID)
	}
}
