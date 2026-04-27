// Package r008 implements R-008: Community D Resonance Suppression.
// A community whose average task D value exceeds the network median by the
// deviation multiplier for a sustained period triggers a FLAG for damping audit.
package r008

import (
	"context"
	"fmt"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID                     = "R-008"
	Category                   = "credit_arbitrage"
	Trigger                    = "periodic_audit"
	DefaultPeriodDays          = 30
	DefaultDeviationMultiplier = 1.5
)

// Input holds the fields checked by R-008 (from R-008.yaml `inputs`).
// All statistical values are pre-computed by the caller (scheduler/aggregator).
type Input struct {
	CommunityID         string
	PeriodDays          int // 0 → DefaultPeriodDays (30)
	CommunityAvgD       float64
	NetworkMedianD      float64
	DeviationMultiplier float64 // 0 → DefaultDeviationMultiplier (1.5)
	SustainedDays       int     // number of consecutive days condition has been met
}

// Result is the outcome of a single R-008 evaluation.
type Result struct {
	Flagged    bool
	AuditEvent audit.Event
}

// Check evaluates R-008. No DB access required — caller supplies pre-computed stats.
func Check(_ context.Context, _ *pgxpool.Pool, in Input) (Result, error) {
	periodDays := in.PeriodDays
	if periodDays == 0 {
		periodDays = DefaultPeriodDays
	}
	multiplier := in.DeviationMultiplier
	if multiplier == 0 {
		multiplier = DefaultDeviationMultiplier
	}

	b := audit.NewBuilder(RuleID, Category, Trigger).WithSeverity("flag")

	threshold := in.NetworkMedianD * multiplier
	if in.CommunityAvgD > threshold && in.SustainedDays >= periodDays {
		pattern := fmt.Sprintf(
			"community %s: avg_D=%.4f > network_median_D=%.4f × %.1f (=%.4f) sustained %d/%d days",
			in.CommunityID,
			in.CommunityAvgD, in.NetworkMedianD, multiplier, threshold,
			in.SustainedDays, periodDays,
		)
		ev := b.BuildFlag("community", in.CommunityID, pattern, []string{"community_admin"})
		return Result{Flagged: true, AuditEvent: ev}, nil
	}

	ev := b.BuildPass("community", in.CommunityID)
	return Result{Flagged: false, AuditEvent: ev}, nil
}
