package r004_test

import (
	"context"
	"testing"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r004"
)

// ── Pure-logic tests (no DB required) ────────────────────────────────────────

func TestCheck_ValidOrder_Pass(t *testing.T) {
	now := time.Now().UTC()
	in := r004.Input{
		TaskID:       "task-001",
		StartTimeUTC: now,
		EndTimeUTC:   now.Add(time.Hour),
	}
	res, err := r004.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", res.Event.Verdict.Result)
	}
}

func TestCheck_SameTimestamp_Pass(t *testing.T) {
	now := time.Now().UTC()
	in := r004.Input{
		TaskID:       "task-002",
		StartTimeUTC: now,
		EndTimeUTC:   now,
	}
	res, err := r004.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// start == end is not start > end, so PASS
	if res.Event.Verdict.Result != audit.VerdictPass {
		t.Errorf("equal timestamps should PASS; got %q", res.Event.Verdict.Result)
	}
}

func TestCheck_InvertedTimestamps_Block(t *testing.T) {
	now := time.Now().UTC()
	in := r004.Input{
		TaskID:       "task-003",
		StartTimeUTC: now.Add(time.Hour), // start is AFTER end
		EndTimeUTC:   now,
	}
	res, err := r004.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Event.Verdict.Result != audit.VerdictBlock {
		t.Errorf("verdict: got %q, want BLOCK", res.Event.Verdict.Result)
	}
	if !res.Event.Verdict.AutoActioned {
		t.Error("BLOCK event must set auto_actioned=true")
	}
	if res.Event.Evidence.MatchedPattern == nil {
		t.Error("BLOCK event must include a matched_pattern")
	}
	if !res.Event.ActionTaken.AppealEligible {
		t.Error("BLOCK event must be appeal_eligible")
	}
}

func TestCheck_RuleMetadata(t *testing.T) {
	now := time.Now().UTC()
	in := r004.Input{
		TaskID:       "task-x",
		StartTimeUTC: now,
		EndTimeUTC:   now.Add(time.Hour),
	}
	res, _ := r004.Check(context.Background(), nil, in)
	if res.Event.Rule.RuleID != r004.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.Event.Rule.RuleID, r004.RuleID)
	}
	if res.Event.Rule.Version != "1.0" {
		t.Errorf("version: got %q, want 1.0", res.Event.Rule.Version)
	}
	if res.Event.Scope.Trigger != r004.Trigger {
		t.Errorf("trigger: got %q, want %q", res.Event.Scope.Trigger, r004.Trigger)
	}
}
