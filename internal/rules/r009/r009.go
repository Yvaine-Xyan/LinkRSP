// Package r009 implements R-009: V-Decay Consecutive V=0 Warning.
// A UID with 10 or more consecutive V=0 self-attestations triggers a
// V-Decay warn, prompting the user to upgrade their verification path.
package r009

import (
	"context"
	"fmt"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID           = "R-009"
	Category         = "credit_arbitrage"
	Trigger          = "post_execution"
	DefaultThreshold = 10
)

// Input holds the fields checked by R-009 (from R-009.yaml `inputs`).
// If ConsecutiveV0Count is non-zero it is used directly instead of querying
// the database — useful for unit testing.
type Input struct {
	UID                string
	ConsecutiveV0Count int // pre-computed override; 0 → query DB
	Threshold          int // 0 → DefaultThreshold (10)
}

// Result is the outcome of a single R-009 evaluation.
type Result struct {
	Warned             bool
	ConsecutiveV0Count int
	AuditEvent         audit.Event
}

// Check evaluates R-009, querying the consecutive V=0 count for the UID
// unless ConsecutiveV0Count is pre-supplied.
func Check(ctx context.Context, pool *pgxpool.Pool, in Input) (Result, error) {
	threshold := in.Threshold
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	count := in.ConsecutiveV0Count
	if count == 0 {
		var err error
		count, err = queryConsecutiveV0(ctx, pool, in.UID)
		if err != nil {
			return Result{}, fmt.Errorf("r009: query consecutive v0: %w", err)
		}
	}

	b := audit.NewBuilder(RuleID, Category, Trigger).WithSeverity("warn")

	if count >= threshold {
		pattern := fmt.Sprintf(
			"uid %s: %d consecutive V=0 attestations >= threshold %d",
			in.UID, count, threshold,
		)
		ev := b.BuildWarn("user", in.UID, pattern, []string{"submitter"})
		return Result{Warned: true, ConsecutiveV0Count: count, AuditEvent: ev}, nil
	}

	ev := b.BuildPass("user", in.UID)
	return Result{Warned: false, ConsecutiveV0Count: count, AuditEvent: ev}, nil
}

// queryConsecutiveV0 counts V=0 attestations for the UID that occur after
// the most recent V>0 attestation (i.e., the current unbroken streak).
// attestations has no uid column; UID is resolved via tasks.uid_submitter.
func queryConsecutiveV0(ctx context.Context, pool *pgxpool.Pool, uid string) (int, error) {
	const q = `
		SELECT COUNT(*)::INT
		FROM   attestations a
		JOIN   tasks t ON a.task_id = t.task_id
		WHERE  t.uid_submitter = $1
		  AND  a.verification_level = 0
		  AND  a.timestamp_utc > COALESCE(
		           (SELECT MAX(a2.timestamp_utc)
		            FROM   attestations a2
		            JOIN   tasks t2 ON a2.task_id = t2.task_id
		            WHERE  t2.uid_submitter = $1
		              AND  a2.verification_level > 0),
		           '1970-01-01'::TIMESTAMPTZ
		       )`

	var count int
	err := pool.QueryRow(ctx, q, uid).Scan(&count)
	return count, err
}
