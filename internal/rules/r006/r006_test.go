package r006_test

import (
	"context"
	"testing"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r006"
)

// ── Pure-logic tests (no DB required) ────────────────────────────────────────
// CreditsSeries is pre-supplied to bypass the DB query.

func TestCheck_InsufficientData_Pass(t *testing.T) {
	// nil triggers DB path; use explicit slices for pre-computed unit tests.
	cases := []struct {
		name   string
		series []float64
	}{
		{"empty series", []float64{}},
		{"one point", []float64{10}},
		{"two points", []float64{10, 20}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := r006.Input{
				UID:                   "uid-alice",
				CreditsSeries:         tc.series,
				AccelerationThreshold: 5,
			}
			res, err := r006.Check(context.Background(), nil, in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Flagged {
				t.Error("insufficient data: should not flag")
			}
			if !res.InsufficientData {
				t.Error("should report InsufficientData=true")
			}
			if res.AuditEvent.Verdict.Result != audit.VerdictPass {
				t.Errorf("verdict: got %q, want PASS", res.AuditEvent.Verdict.Result)
			}
		})
	}
}

func TestCheck_LinearGrowth_Pass(t *testing.T) {
	// Linear series: Δ²=0 everywhere → below any positive threshold
	in := r006.Input{
		UID:                   "uid-bob",
		CreditsSeries:         []float64{10, 20, 30, 40, 50},
		AccelerationThreshold: 1,
	}
	res, err := r006.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("linear growth (Δ²=0): should not flag")
	}
	if res.AccelerationValue != 0 {
		t.Errorf("linear series: max d²=%.4f, want 0", res.AccelerationValue)
	}
}

func TestCheck_ConstantSeries_Pass(t *testing.T) {
	in := r006.Input{
		UID:                   "uid-carol",
		CreditsSeries:         []float64{50, 50, 50, 50},
		AccelerationThreshold: 1,
	}
	res, err := r006.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("constant series (Δ²=0): should not flag")
	}
}

func TestCheck_ExponentialBurst_Flag(t *testing.T) {
	// Series: 10, 10, 10, 100 → Δ²[3] = 100 - 2*10 + 10 = 90
	in := r006.Input{
		UID:                   "uid-dave",
		CreditsSeries:         []float64{10, 10, 10, 100},
		AccelerationThreshold: 50,
	}
	res, err := r006.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Flagged {
		t.Error("burst gives Δ²=90 > threshold 50: should flag")
	}
	if res.AuditEvent.Verdict.Result != audit.VerdictFlag {
		t.Errorf("verdict: got %q, want FLAG", res.AuditEvent.Verdict.Result)
	}
	if res.AuditEvent.Verdict.Severity != "flag" {
		t.Errorf("severity: got %q, want flag", res.AuditEvent.Verdict.Severity)
	}
	if res.AuditEvent.Evidence.MatchedPattern == nil {
		t.Error("FLAG event must include matched_pattern")
	}
	if res.AuditEvent.Verdict.AutoActioned {
		t.Error("FLAG event must not be auto_actioned")
	}
	if res.AccelerationValue != 90 {
		t.Errorf("AccelerationValue: got %.4f, want 90", res.AccelerationValue)
	}
}

func TestCheck_ExactThreshold_Pass(t *testing.T) {
	// Δ² = threshold → condition is strict >, so should PASS
	// series: 0, 5, 15 → Δ²[2] = 15 - 2*5 + 0 = 5
	in := r006.Input{
		UID:                   "uid-eve",
		CreditsSeries:         []float64{0, 5, 15},
		AccelerationThreshold: 5,
	}
	res, err := r006.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Error("Δ²=5 == threshold 5: should PASS (strict >)")
	}
}

func TestCheck_MaxSecondDerivative_PicksMax(t *testing.T) {
	// series: 0, 0, 0, 0, 200 → Δ²[4] = 200 - 2*0 + 0 = 200 (max among all)
	in := r006.Input{
		UID:                   "uid-frank",
		CreditsSeries:         []float64{0, 0, 0, 0, 200},
		AccelerationThreshold: 100,
	}
	res, err := r006.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Flagged {
		t.Error("max Δ²=200 > threshold 100: should flag")
	}
	if res.AccelerationValue != 200 {
		t.Errorf("AccelerationValue: got %.4f, want 200", res.AccelerationValue)
	}
}

func TestCheck_NegativeAcceleration_Pass(t *testing.T) {
	// Decelerating series should never flag
	// 100, 50, 20 → Δ²[2] = 20 - 2*50 + 100 = 20
	in := r006.Input{
		UID:                   "uid-grace",
		CreditsSeries:         []float64{100, 50, 20},
		AccelerationThreshold: 50,
	}
	res, err := r006.Check(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Flagged {
		t.Errorf("Δ²=20 < threshold 50: should not flag; AccelerationValue=%.4f", res.AccelerationValue)
	}
}

func TestCheck_FlagSeverity_PassEvent(t *testing.T) {
	in := r006.Input{
		UID:                   "uid-hank",
		CreditsSeries:         []float64{10, 20, 30},
		AccelerationThreshold: 100,
	}
	res, _ := r006.Check(context.Background(), nil, in)
	if res.AuditEvent.Verdict.Severity != "flag" {
		t.Errorf("PASS from flag rule: severity got %q, want flag", res.AuditEvent.Verdict.Severity)
	}
}

func TestCheck_RuleMetadata(t *testing.T) {
	in := r006.Input{
		UID:                   "uid-meta",
		CreditsSeries:         []float64{1, 2, 3},
		AccelerationThreshold: 100,
	}
	res, _ := r006.Check(context.Background(), nil, in)
	if res.AuditEvent.Rule.RuleID != r006.RuleID {
		t.Errorf("rule_id: got %q, want %q", res.AuditEvent.Rule.RuleID, r006.RuleID)
	}
	if res.AuditEvent.Scope.Trigger != r006.Trigger {
		t.Errorf("trigger: got %q, want %q", res.AuditEvent.Scope.Trigger, r006.Trigger)
	}
	if res.AuditEvent.Subject.Type != "user" {
		t.Errorf("subject.type: got %q, want user", res.AuditEvent.Subject.Type)
	}
}
