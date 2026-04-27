// Package r006 implements R-006: Credit Acceleration Anomaly (d²C/dt²).
// A UID whose credit output shows a second derivative exceeding the configured
// threshold — indicating abnormal acceleration — is flagged for audit.
// Requires at least 3 data points; fewer → PASS (insufficient data).
package r006

import (
	"context"
	"fmt"
	"math"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID            = "R-006"
	Category          = "credit_arbitrage"
	Trigger           = "post_execution"
	DefaultWindowDays = 7 // days of daily buckets to query from DB
)

// Input holds the fields checked by R-006 (from R-006.yaml `inputs`).
// CreditsSeries is a time-ordered slice of per-period credit sums (e.g., daily).
// If CreditsSeries is empty, the last WindowDays daily buckets are queried from
// ledger_entries. AccelerationThreshold is required; there is no default because
// it is derived from a per-community baseline the caller supplies.
type Input struct {
	UID                   string
	CommunityID           string
	CreditsSeries         []float64 // pre-computed; nil → query DB; []float64{} → use as-is (may yield InsufficientData)
	AccelerationThreshold float64
	WindowDays            int // 0 → DefaultWindowDays (used only when querying DB)
}

// Result is the outcome of a single R-006 evaluation.
type Result struct {
	Flagged           bool
	AccelerationValue float64 // max second difference found in the series
	InsufficientData  bool    // fewer than 3 points — rule could not fire
	AuditEvent        audit.Event
}

// Check evaluates R-006. Queries daily credit buckets when CreditsSeries is empty.
func Check(ctx context.Context, pool *pgxpool.Pool, in Input) (Result, error) {
	series := in.CreditsSeries
	if series == nil {
		windowDays := in.WindowDays
		if windowDays == 0 {
			windowDays = DefaultWindowDays
		}
		var err error
		series, err = queryDailySeries(ctx, pool, in.UID, windowDays)
		if err != nil {
			return Result{}, fmt.Errorf("r006: query daily series: %w", err)
		}
	}

	b := audit.NewBuilder(RuleID, Category, Trigger).WithSeverity("flag")

	if len(series) < 3 {
		ev := b.BuildPass("user", in.UID)
		return Result{InsufficientData: true, AuditEvent: ev}, nil
	}

	accel := maxSecondDerivative(series)

	if accel > in.AccelerationThreshold {
		pattern := fmt.Sprintf(
			"uid %s community %s: max d²C/dt²=%.4f exceeds threshold=%.4f over %d-period series",
			in.UID, in.CommunityID, accel, in.AccelerationThreshold, len(series),
		)
		ev := b.BuildFlag("user", in.UID, pattern, []string{"submitter"})
		return Result{Flagged: true, AccelerationValue: accel, AuditEvent: ev}, nil
	}

	ev := b.BuildPass("user", in.UID)
	return Result{Flagged: false, AccelerationValue: accel, AuditEvent: ev}, nil
}

// maxSecondDerivative returns the maximum second difference from the time series.
// Second differences: Δ²[i] = series[i] - 2*series[i-1] + series[i-2]
// Requires len(series) >= 3.
func maxSecondDerivative(series []float64) float64 {
	max := math.Inf(-1)
	for i := 2; i < len(series); i++ {
		d2 := series[i] - 2*series[i-1] + series[i-2]
		if d2 > max {
			max = d2
		}
	}
	return max
}

// queryDailySeries returns per-day credit sums for the UID over the last windowDays days.
// Days with no ledger entries contribute a zero (filled via generate_series).
func queryDailySeries(ctx context.Context, pool *pgxpool.Pool, uid string, windowDays int) ([]float64, error) {
	const q = `
		WITH day_series AS (
		    SELECT generate_series(
		        DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC') - ($2 - 1) * INTERVAL '1 day',
		        DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC'),
		        INTERVAL '1 day'
		    ) AS day
		),
		daily_credits AS (
		    SELECT DATE_TRUNC('day', created_at_utc AT TIME ZONE 'UTC') AS day,
		           SUM(credits_delta)::FLOAT8 AS total
		    FROM   ledger_entries
		    WHERE  uid = $1
		      AND  created_at_utc >= DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC') - ($2 - 1) * INTERVAL '1 day'
		    GROUP  BY 1
		)
		SELECT COALESCE(dc.total, 0)
		FROM   day_series ds
		LEFT   JOIN daily_credits dc ON ds.day = dc.day
		ORDER  BY ds.day ASC`

	rows, err := pool.Query(ctx, q, uid, windowDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		series = append(series, v)
	}
	return series, rows.Err()
}
