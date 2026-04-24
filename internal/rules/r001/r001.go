// Package r001 implements R-001: Physical Paradox Detection.
// A single UID cannot be physically present at two different locations
// simultaneously. If two V>=1 tasks overlap in time with different
// location_hash values, the current settlement is blocked.
package r001

import (
	"context"
	"fmt"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID   = "R-001"
	Category = "physical_paradox"
	Trigger  = "post_execution"
)

// Input holds the fields checked by R-001 (from R-001.yaml `inputs`).
type Input struct {
	UID               string
	TaskID            string
	StartTimeUTC      time.Time
	EndTimeUTC        time.Time
	LocationHash      string
	VerificationLevel int // 0 / 1 / 2
}

// Result is the outcome of a single R-001 evaluation.
type Result struct {
	Event           audit.Event
	ConflictingTask *ConflictDetail // nil when verdict is PASS
}

type ConflictDetail struct {
	TaskID          string
	OverlapMinutes  float64
	LocationHash    string
}

// Check runs R-001 against the database and returns a BLOCK or PASS event.
// All V>=1 tasks for the same UID are queried; if any overlaps with a
// different location_hash the rule fires.
func Check(ctx context.Context, pool *pgxpool.Pool, in Input) (Result, error) {
	if in.VerificationLevel < 1 {
		// Rule only applies to V>=1 tasks — auto-PASS for V=0.
		ev := audit.NewBuilder(RuleID, Category, Trigger).BuildPass("task", in.TaskID)
		return Result{Event: ev}, nil
	}

	row, err := findConflict(ctx, pool, in)
	if err != nil {
		return Result{}, fmt.Errorf("r001: query conflict: %w", err)
	}

	builder := audit.NewBuilder(RuleID, Category, Trigger)

	if row == nil {
		ev := builder.BuildPass("task", in.TaskID)
		return Result{Event: ev}, nil
	}

	// BLOCK: physical paradox detected
	pattern := fmt.Sprintf(
		"uid %s has overlapping V>=1 task %s (%.1f min) at different location",
		in.UID, row.TaskID, row.OverlapMinutes,
	)
	ev := builder.BuildBlock(
		"task", in.TaskID, &row.TaskID,
		pattern,
		[]string{"submitter"},
	)
	ev.Evidence.ConflictingIDs = []string{row.TaskID}

	return Result{Event: ev, ConflictingTask: row}, nil
}

// findConflict queries for any V>=1 task by the same UID that overlaps in time
// and has a different location_hash. Returns nil if no conflict is found.
func findConflict(ctx context.Context, pool *pgxpool.Pool, in Input) (*ConflictDetail, error) {
	const q = `
		SELECT
			t.task_id,
			t.location_hash,
			EXTRACT(EPOCH FROM (
				LEAST(t.end_time_utc, $4) - GREATEST(t.start_time_utc, $3)
			)) / 60.0 AS overlap_minutes
		FROM tasks t
		JOIN attestations a ON a.task_id = t.task_id
		WHERE t.uid_submitter = $1
		  AND t.task_id      != $2
		  AND a.verification_level >= 1
		  AND t.start_time_utc < $4
		  AND t.end_time_utc   > $3
		  AND t.location_hash IS NOT NULL
		  AND t.location_hash != $5
		ORDER BY overlap_minutes DESC
		LIMIT 1`

	rows, err := pool.Query(ctx, q,
		in.UID, in.TaskID, in.StartTimeUTC, in.EndTimeUTC, in.LocationHash,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}

	var d ConflictDetail
	if err := rows.Scan(&d.TaskID, &d.LocationHash, &d.OverlapMinutes); err != nil {
		return nil, err
	}
	return &d, nil
}
