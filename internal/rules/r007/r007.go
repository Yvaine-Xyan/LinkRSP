// Package r007 implements R-007: IPO ΣW_risk Ceiling.
// A community ruleset whose theoretical ΣW_risk peak exceeds the Clip
// upper bound (3.0) is blocked from the IPO (升S) process.
package r007

import (
	"context"
	"fmt"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID                = "R-007"
	Category              = "credit_arbitrage"
	Trigger               = "ipo_scan"
	DefaultClipUpperBound = 3.0
)

// Input holds the fields checked by R-007 (from R-007.yaml `inputs`).
type Input struct {
	CommunityID             string
	ProposedRulesetID       string
	SumWRiskTheoreticalPeak float64
	ClipUpperBound          float64 // 0 → DefaultClipUpperBound (3.0)
}

// Result is the outcome of a single R-007 evaluation.
type Result struct {
	Blocked    bool
	AuditEvent audit.Event
}

// Check evaluates R-007. No DB access required — pure arithmetic comparison.
func Check(_ context.Context, _ *pgxpool.Pool, in Input) (Result, error) {
	clipUpper := in.ClipUpperBound
	if clipUpper == 0 {
		clipUpper = DefaultClipUpperBound
	}

	b := audit.NewBuilder(RuleID, Category, Trigger)

	if in.SumWRiskTheoreticalPeak > clipUpper {
		pattern := fmt.Sprintf(
			"community %s ruleset %s: sum_w_risk_theoretical_peak=%.4f exceeds clip_upper_bound=%.4f",
			in.CommunityID, in.ProposedRulesetID,
			in.SumWRiskTheoreticalPeak, clipUpper,
		)
		ev := b.BuildBlock("community", in.CommunityID, nil, pattern, []string{"submitter"})
		return Result{Blocked: true, AuditEvent: ev}, nil
	}

	ev := b.BuildPass("community", in.CommunityID)
	return Result{Blocked: false, AuditEvent: ev}, nil
}
