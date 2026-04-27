package r008_test

import (
	"context"
	"testing"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r008"
)

func TestCheck_BelowThreshold_Pass(t *testing.T) {
	in := r008.Input{
		CommunityID:    "comm-001",
		CommunityAvgD:  1.4,
		NetworkMedianD: 1.0,
		SustainedDays:  35,
	}
	res, err := r008.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("avg_D=1.4 < 1.0×1.5=1.5: should not flag")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", res.AuditEvent.Verdict.Result)
	}
}

func TestCheck_AboveThreshold_NotSustained_Pass(t *testing.T) {
	in := r008.Input{
		CommunityID:    "comm-002",
		CommunityAvgD:  1.8,
		NetworkMedianD: 1.0,
		SustainedDays:  15, // only 15 days, need 30
	}
	res, err := r008.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("condition met but sustained only 15/30 days: should not flag")
	}
}

func TestCheck_AboveThreshold_Sustained_Flag(t *testing.T) {
	in := r008.Input{
		CommunityID:    "comm-003",
		CommunityAvgD:  1.8,
		NetworkMedianD: 1.0,
		SustainedDays:  30,
	}
	res, err := r008.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Flagged {
		t.Error("avg_D=1.8 > 1.0×1.5=1.5 for 30 days: should flag")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictFlag {
		t.Errorf("verdict: got %q, want FLAG", res.AuditEvent.Verdict.Result)
	}
	if res.AuditEvent.Verdict.Severity != "flag" {
		t.Errorf("severity: got %q, want flag", res.AuditEvent.Verdict.Severity)
	}
	if res.AuditEvent.Verdict.AutoActioned {
		t.Error("FLAG event must not be auto_actioned")
	}
	if res.AuditEvent.Evidence.MatchedPattern == nil {
		t.Error("FLAG event must include matched_pattern")
	}
}

func TestCheck_ExactThreshold_Pass(t *testing.T) {
	// condition is strict >; exactly at threshold should PASS
	in := r008.Input{
		CommunityID:    "comm-004",
		CommunityAvgD:  1.5,
		NetworkMedianD: 1.0,
		SustainedDays:  30,
	}
	res, err := r008.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("exactly at threshold (1.5 == 1.0×1.5): should PASS (strict >)")
	}
}

func TestCheck_CustomMultiplier_Flag(t *testing.T) {
	in := r008.Input{
		CommunityID:         "comm-005",
		CommunityAvgD:       1.3,
		NetworkMedianD:      1.0,
		DeviationMultiplier: 1.2,
		SustainedDays:       30,
	}
	res, err := r008.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Flagged {
		t.Error("1.3 > 1.0×1.2=1.2 for 30 days with custom multiplier: should flag")
	}
}

func TestCheck_CustomPeriod_Flag(t *testing.T) {
	in := r008.Input{
		CommunityID:    "comm-006",
		CommunityAvgD:  1.8,
		NetworkMedianD: 1.0,
		PeriodDays:     15,
		SustainedDays:  15,
	}
	res, err := r008.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Flagged {
		t.Error("condition met for custom period 15 days: should flag")
	}
}

func TestCheck_FlagSeverity_PassEvent(t *testing.T) {
	// PASS events from a flag-severity rule must carry severity "flag".
	in := r008.Input{
		CommunityID:    "comm-007",
		CommunityAvgD:  1.0,
		NetworkMedianD: 1.0,
		SustainedDays:  0,
	}
	res, _ := r008.Check(context.Background(), nil, in)
	if res.AuditEvent.Verdict.Severity != "flag" {
		t.Errorf("PASS from flag rule: severity got %q, want flag", res.AuditEvent.Verdict.Severity)
	}
}

func TestCheck_RuleMetadata(t *testing.T) {
	in := r008.Input{
		CommunityID:    "comm-meta",
		CommunityAvgD:  1.0,
		NetworkMedianD: 1.0,
	}
	res, _ := r008.Check(context.Background(), nil, in)
	if res.AuditEvent.Rule.RuleID != r008.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.AuditEvent.Rule.RuleID, r008.RuleID)
	}
	if res.AuditEvent.Scope.Trigger != r008.Trigger {
		t.Errorf("trigger: got %q, want %q", res.AuditEvent.Scope.Trigger, r008.Trigger)
	}
	if res.AuditEvent.Subject.Type != "community" {
		t.Errorf("subject.type: got %q, want community", res.AuditEvent.Subject.Type)
	}
}
