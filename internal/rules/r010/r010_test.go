package r010_test

import (
	"context"
	"testing"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r010"
)

var genesisEnd = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

func TestCheck_BeforeGenesisEnds_Pass(t *testing.T) {
	in := r010.Input{
		UID:                       "uid-alice",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd.Add(-1 * time.Hour), // still in genesis
		UserCreditsInWindow:       1000,
		NetworkP99CreditsInWindow: 500,
	}
	res, err := r010.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("before genesis ends: should not flag")
	}
	if res.InPostGenesisWindow {
		t.Error("before genesis ends: InPostGenesisWindow should be false")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictPass {
		t.Errorf("verdict: got %q, want PASS", res.AuditEvent.Verdict.Result)
	}
}

func TestCheck_AfterWindow_Pass(t *testing.T) {
	in := r010.Input{
		UID:                       "uid-bob",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd.Add(8 * 24 * time.Hour), // 8 days after, outside 7-day window
		UserCreditsInWindow:       1000,
		NetworkP99CreditsInWindow: 500,
	}
	res, err := r010.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("8 days after genesis end (outside 7-day window): should not flag")
	}
	if res.InPostGenesisWindow {
		t.Error("outside window: InPostGenesisWindow should be false")
	}
}

func TestCheck_InWindow_BelowP99_Pass(t *testing.T) {
	in := r010.Input{
		UID:                       "uid-charlie",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd.Add(3 * 24 * time.Hour),
		UserCreditsInWindow:       400,
		NetworkP99CreditsInWindow: 500,
	}
	res, err := r010.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("credits 400 < p99 500: should not flag")
	}
	if !res.InPostGenesisWindow {
		t.Error("3 days after genesis: InPostGenesisWindow should be true")
	}
}

func TestCheck_InWindow_AboveP99_Flag(t *testing.T) {
	in := r010.Input{
		UID:                       "uid-dave",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd.Add(3 * 24 * time.Hour),
		UserCreditsInWindow:       1000,
		NetworkP99CreditsInWindow: 500,
	}
	res, err := r010.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Flagged {
		t.Error("credits 1000 > p99 500 within window: should flag")
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

func TestCheck_ExactP99_Pass(t *testing.T) {
	// condition is strict >; exactly at p99 should PASS
	in := r010.Input{
		UID:                       "uid-eve",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd.Add(1 * 24 * time.Hour),
		UserCreditsInWindow:       500,
		NetworkP99CreditsInWindow: 500,
	}
	res, err := r010.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("credits == p99: should PASS (strict >)")
	}
}

func TestCheck_WindowBoundary_Included(t *testing.T) {
	// CurrentTime == GenesisEndTime should be in window
	in := r010.Input{
		UID:                       "uid-frank",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd,
		UserCreditsInWindow:       1000,
		NetworkP99CreditsInWindow: 500,
	}
	res, err := r010.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.InPostGenesisWindow {
		t.Error("CurrentTime == GenesisEndTime should be in window")
	}
}

func TestCheck_CustomLookback_Flag(t *testing.T) {
	in := r010.Input{
		UID:                       "uid-grace",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd.Add(9 * 24 * time.Hour),
		LookbackDays:              10,
		UserCreditsInWindow:       1000,
		NetworkP99CreditsInWindow: 500,
	}
	res, err := r010.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Flagged {
		t.Error("9 days within custom 10-day window with credits > p99: should flag")
	}
}

func TestCheck_FlagSeverity_PassEvent(t *testing.T) {
	in := r010.Input{
		UID:                       "uid-hank",
		GenesisEndTime:            genesisEnd,
		CurrentTime:               genesisEnd.Add(3 * 24 * time.Hour),
		UserCreditsInWindow:       100,
		NetworkP99CreditsInWindow: 500,
	}
	res, _ := r010.Check(context.Background(), nil, in)
	if res.AuditEvent.Verdict.Severity != "flag" {
		t.Errorf("PASS from flag rule: severity got %q, want flag", res.AuditEvent.Verdict.Severity)
	}
}

func TestCheck_RuleMetadata(t *testing.T) {
	in := r010.Input{
		UID:            "uid-meta",
		GenesisEndTime: genesisEnd,
		CurrentTime:    genesisEnd.Add(-1 * time.Hour),
	}
	res, _ := r010.Check(context.Background(), nil, in)
	if res.AuditEvent.Rule.RuleID != r010.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.AuditEvent.Rule.RuleID, r010.RuleID)
	}
	if res.AuditEvent.Scope.Trigger != r010.Trigger {
		t.Errorf("trigger: got %q, want %q", res.AuditEvent.Scope.Trigger, r010.Trigger)
	}
	if res.AuditEvent.Subject.Type != "user" {
		t.Errorf("subject.type: got %q, want user", res.AuditEvent.Subject.Type)
	}
}
