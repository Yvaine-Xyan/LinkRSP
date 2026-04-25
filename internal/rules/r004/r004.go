// Package r004 implements R-004: Inverted Timestamp.
// A task whose start_time_utc is after end_time_utc is a deterministic
// data error and is blocked before execution.
package r004

import (
	"context"
	"fmt"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID   = "R-004"
	Category = "data_integrity"
	Trigger  = "pre_execution"
)

// Input holds the fields checked by R-004 (from R-004.yaml `inputs`).
type Input struct {
	TaskID       string
	StartTimeUTC time.Time
	EndTimeUTC   time.Time
}

// Result is the outcome of a single R-004 evaluation.
type Result struct {
	Event audit.Event
}

// Check evaluates R-004. pool is unused (pure time comparison) but accepted
// for API consistency with other rule packages.
func Check(_ context.Context, _ *pgxpool.Pool, in Input) (Result, error) {
	builder := audit.NewBuilder(RuleID, Category, Trigger)

	if !in.StartTimeUTC.After(in.EndTimeUTC) {
		ev := builder.BuildPass("task", in.TaskID)
		return Result{Event: ev}, nil
	}

	pattern := fmt.Sprintf(
		"task %s: start_time %s is after end_time %s",
		in.TaskID,
		in.StartTimeUTC.UTC().Format(time.RFC3339),
		in.EndTimeUTC.UTC().Format(time.RFC3339),
	)
	ev := builder.BuildBlock("task", in.TaskID, nil, pattern, []string{"submitter"})
	return Result{Event: ev}, nil
}
