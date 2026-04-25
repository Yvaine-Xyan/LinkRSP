// Package r005 implements R-005: Daily Hour Ceiling.
// A single UID accumulating more than 960 minutes (16 h) of physical
// task time in any rolling 24-hour window exceeds sustainable human
// capacity. The rule emits a FLAG/WARN event for operator review.
package r005

import (
	"context"
	"fmt"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID              = "R-005"
	Category            = "physical_paradox"
	Trigger             = "post_execution"
	DefaultMaxMinutes   = 960.0  // 16 hours
	windowDuration      = 24 * time.Hour
)

// Input holds the fields checked by R-005 (from R-005.yaml `inputs`).
// WindowEndUTC is the reference point for the 24-hour rolling window
// (typically the current task's end_time_utc).
// If TotalMinutesInWindow is non-zero it is used directly instead of
// querying the database — useful for unit testing.
type Input struct {
	UID                  string
	TaskID               string
	WindowEndUTC         time.Time
	TotalMinutesInWindow float64 // pre-computed override; 0 → query DB
	MaxMinutes           float64 // 0 → DefaultMaxMinutes (960)
}

// Result is the outcome of a single R-005 evaluation.
type Result struct {
	Event               audit.Event
	TotalMinutesInWindow float64
}

// Check evaluates R-005, querying total task minutes in the 24-hour window
// ending at in.WindowEndUTC unless TotalMinutesInWindow is pre-supplied.
func Check(ctx context.Context, pool *pgxpool.Pool, in Input) (Result, error) {
	max := in.MaxMinutes
	if max == 0 {
		max = DefaultMaxMinutes
	}

	total := in.TotalMinutesInWindow
	if total == 0 {
		var err error
		total, err = queryWindowMinutes(ctx, pool, in)
		if err != nil {
			return Result{}, fmt.Errorf("r005: query window minutes: %w", err)
		}
	}

	builder := audit.NewBuilder(RuleID, Category, Trigger).WithSeverity("warn")

	if total <= max {
		ev := builder.BuildPass("task", in.TaskID)
		return Result{Event: ev, TotalMinutesInWindow: total}, nil
	}

	windowStart := in.WindowEndUTC.Add(-windowDuration)
	pattern := fmt.Sprintf(
		"uid %s: %.0f min in 24h window [%s, %s] exceeds ceiling %.0f min (16 h)",
		in.UID, total,
		windowStart.UTC().Format("2006-01-02T15:04Z"),
		in.WindowEndUTC.UTC().Format("2006-01-02T15:04Z"),
		max,
	)
	ev := builder.BuildWarn("task", in.TaskID, pattern, []string{"submitter"})
	return Result{Event: ev, TotalMinutesInWindow: total}, nil
}

// queryWindowMinutes returns the sum of task durations (in minutes) for
// the given UID within the 24-hour window ending at in.WindowEndUTC.
func queryWindowMinutes(ctx context.Context, pool *pgxpool.Pool, in Input) (float64, error) {
	windowStart := in.WindowEndUTC.Add(-windowDuration)

	const q = `
		SELECT COALESCE(SUM(
			EXTRACT(EPOCH FROM (end_time_utc - start_time_utc)) / 60.0
		), 0)
		FROM   tasks
		WHERE  uid_submitter   = $1
		  AND  start_time_utc >= $2
		  AND  start_time_utc  < $3`

	var total float64
	err := pool.QueryRow(ctx, q, in.UID, windowStart, in.WindowEndUTC).Scan(&total)
	return total, err
}
