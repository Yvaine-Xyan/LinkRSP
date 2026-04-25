// Package r003 implements R-003: Task Duration Ceiling.
// A single task claiming more than 1440 minutes (24 h) is physically
// implausible and is blocked before execution.
package r003

import (
	"context"
	"fmt"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID            = "R-003"
	Category          = "physical_paradox"
	Trigger           = "pre_execution"
	DefaultMaxMinutes = 1440.0
)

// Input holds the fields checked by R-003 (from R-003.yaml `inputs`).
type Input struct {
	TaskID          string
	StartTimeUTC    time.Time
	EndTimeUTC      time.Time
	DeclaredMinutes float64 // caller-supplied override; 0 → use computed value
	MaxMinutes      float64 // 0 → DefaultMaxMinutes (1440)
}

// Result is the outcome of a single R-003 evaluation.
type Result struct {
	Event           audit.Event
	ComputedMinutes float64
}

// Check evaluates R-003. pool is unused (pure time arithmetic) but accepted
// for API consistency with other rule packages.
func Check(_ context.Context, _ *pgxpool.Pool, in Input) (Result, error) {
	max := in.MaxMinutes
	if max == 0 {
		max = DefaultMaxMinutes
	}

	computed := in.EndTimeUTC.Sub(in.StartTimeUTC).Minutes()
	declared := in.DeclaredMinutes

	exceeded := computed > max || (declared > 0 && declared > max)

	builder := audit.NewBuilder(RuleID, Category, Trigger)

	if !exceeded {
		ev := builder.BuildPass("task", in.TaskID)
		return Result{Event: ev, ComputedMinutes: computed}, nil
	}

	effective := computed
	if declared > effective {
		effective = declared
	}
	pattern := fmt.Sprintf(
		"task %s duration %.0f min exceeds ceiling %.0f min (24 h)",
		in.TaskID, effective, max,
	)
	ev := builder.BuildBlock("task", in.TaskID, nil, pattern, []string{"submitter"})
	return Result{Event: ev, ComputedMinutes: computed}, nil
}
