package r007_test

import (
	"context"
	"testing"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r007"
)

func TestCheck_BelowCeiling_Pass(t *testing.T) {
	in := r007.Input{
		CommunityID:             "comm-001",
		ProposedRulesetID:       "rs-001",
		SumWRiskTheoreticalPeak: 2.5,
	}
	res, err := r007.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Blocked {
		t.Error("2.5 < 3.0: should not be blocked")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", res.AuditEvent.Verdict.Result)
	}
}

func TestCheck_ExactCeiling_Pass(t *testing.T) {
	in := r007.Input{
		CommunityID:             "comm-002",
		ProposedRulesetID:       "rs-002",
		SumWRiskTheoreticalPeak: 3.0,
	}
	res, err := r007.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Blocked {
		t.Error("exactly 3.0 should PASS (condition is strict >)")
	}
}

func TestCheck_AboveCeiling_Block(t *testing.T) {
	in := r007.Input{
		CommunityID:             "comm-003",
		ProposedRulesetID:       "rs-003",
		SumWRiskTheoreticalPeak: 3.1,
	}
	res, err := r007.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Blocked {
		t.Error("3.1 > 3.0: should be blocked")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictBlock {
		t.Errorf("verdict: got %q, want BLOCK", res.AuditEvent.Verdict.Result)
	}
	if res.AuditEvent.Verdict.Severity != "block" {
		t.Errorf("severity: got %q, want block", res.AuditEvent.Verdict.Severity)
	}
	if !res.AuditEvent.Verdict.AutoActioned {
		t.Error("BLOCK event must be auto_actioned")
	}
	if res.AuditEvent.Evidence.MatchedPattern == nil {
		t.Error("BLOCK event must include matched_pattern")
	}
}

func TestCheck_CustomCeiling_Block(t *testing.T) {
	in := r007.Input{
		CommunityID:             "comm-004",
		ProposedRulesetID:       "rs-004",
		SumWRiskTheoreticalPeak: 2.0,
		ClipUpperBound:          1.5,
	}
	res, err := r007.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Blocked {
		t.Error("2.0 > custom ceiling 1.5: should be blocked")
	}
}

func TestCheck_CustomCeiling_Pass(t *testing.T) {
	in := r007.Input{
		CommunityID:             "comm-005",
		ProposedRulesetID:       "rs-005",
		SumWRiskTheoreticalPeak: 1.4,
		ClipUpperBound:          1.5,
	}
	res, err := r007.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Blocked {
		t.Error("1.4 < custom ceiling 1.5: should not be blocked")
	}
}

func TestCheck_RuleMetadata(t *testing.T) {
	in := r007.Input{
		CommunityID:             "comm-meta",
		ProposedRulesetID:       "rs-meta",
		SumWRiskTheoreticalPeak: 1.0,
	}
	res, _ := r007.Check(context.Background(), nil, in)
	if res.AuditEvent.Rule.RuleID != r007.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.AuditEvent.Rule.RuleID, r007.RuleID)
	}
	if res.AuditEvent.Scope.Trigger != r007.Trigger {
		t.Errorf("trigger: got %q, want %q", res.AuditEvent.Scope.Trigger, r007.Trigger)
	}
	if res.AuditEvent.Subject.Type != "community" {
		t.Errorf("subject.type: got %q, want community", res.AuditEvent.Subject.Type)
	}
}

func TestCheck_AppealEligible_OnBlock(t *testing.T) {
	in := r007.Input{
		CommunityID:             "comm-006",
		ProposedRulesetID:       "rs-006",
		SumWRiskTheoreticalPeak: 5.0,
	}
	res, _ := r007.Check(context.Background(), nil, in)
	if !res.AuditEvent.ActionTaken.AppealEligible {
		t.Error("BLOCK events must be appeal_eligible")
	}
}
