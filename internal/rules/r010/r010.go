// Package r010 implements R-010: Post-Genesis P99 Credit Outlier.
// In the 7-day window immediately after genesis ends, a UID whose total
// credits exceed the network p99 for the same window is flagged for audit.
package r010

import (
	"context"
	"fmt"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID              = "R-010"
	Category            = "credit_arbitrage"
	Trigger             = "post_execution"
	DefaultLookbackDays = 7
)

// Input holds the fields checked by R-010 (from R-010.yaml `inputs`).
// If UserCreditsInWindow or NetworkP99CreditsInWindow is non-zero those
// values are used directly; otherwise both are queried from the database.
type Input struct {
	UID                       string
	GenesisEndTime            time.Time
	CurrentTime               time.Time // zero → time.Now().UTC()
	LookbackDays              int       // 0 → DefaultLookbackDays (7)
	UserCreditsInWindow       float64   // pre-computed; 0 → query DB
	NetworkP99CreditsInWindow float64   // pre-computed; 0 → query DB
}

// Result is the outcome of a single R-010 evaluation.
type Result struct {
	Flagged                   bool
	InPostGenesisWindow       bool
	UserCreditsInWindow       float64
	NetworkP99CreditsInWindow float64
	AuditEvent                audit.Event
}

// Check evaluates R-010. Only fires during the [GenesisEndTime, GenesisEndTime+LookbackDays] window.
func Check(ctx context.Context, pool *pgxpool.Pool, in Input) (Result, error) {
	lookback := in.LookbackDays
	if lookback == 0 {
		lookback = DefaultLookbackDays
	}
	now := in.CurrentTime
	if now.IsZero() {
		now = time.Now().UTC()
	}

	b := audit.NewBuilder(RuleID, Category, Trigger).WithSeverity("flag")

	windowEnd := in.GenesisEndTime.Add(time.Duration(lookback) * 24 * time.Hour)
	inWindow := !now.Before(in.GenesisEndTime) && !now.After(windowEnd)

	if !inWindow {
		ev := b.BuildPass("user", in.UID)
		return Result{InPostGenesisWindow: false, AuditEvent: ev}, nil
	}

	userCredits := in.UserCreditsInWindow
	p99Credits := in.NetworkP99CreditsInWindow
	if userCredits == 0 || p99Credits == 0 {
		var err error
		userCredits, p99Credits, err = queryWindowStats(ctx, pool, in.UID, in.GenesisEndTime, windowEnd)
		if err != nil {
			return Result{}, fmt.Errorf("r010: query window stats: %w", err)
		}
	}

	if userCredits > p99Credits {
		pattern := fmt.Sprintf(
			"uid %s: credits_in_window=%.4f > network_p99=%.4f in %d-day post-genesis window",
			in.UID, userCredits, p99Credits, lookback,
		)
		ev := b.BuildFlag("user", in.UID, pattern, []string{"submitter"})
		return Result{
			Flagged: true, InPostGenesisWindow: true,
			UserCreditsInWindow: userCredits, NetworkP99CreditsInWindow: p99Credits,
			AuditEvent: ev,
		}, nil
	}

	ev := b.BuildPass("user", in.UID)
	return Result{
		Flagged: false, InPostGenesisWindow: true,
		UserCreditsInWindow: userCredits, NetworkP99CreditsInWindow: p99Credits,
		AuditEvent: ev,
	}, nil
}

// queryWindowStats fetches the UID's total credits and the network p99 in [windowStart, windowEnd].
func queryWindowStats(ctx context.Context, pool *pgxpool.Pool, uid string, windowStart, windowEnd time.Time) (userCredits, p99Credits float64, err error) {
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(credits_delta), 0)::FLOAT8
		FROM   ledger_entries
		WHERE  uid = $1
		  AND  created_at_utc >= $2
		  AND  created_at_utc <= $3`,
		uid, windowStart, windowEnd,
	).Scan(&userCredits)
	if err != nil {
		return 0, 0, err
	}

	err = pool.QueryRow(ctx, `
		SELECT COALESCE(
		    PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY total_credits),
		    0
		)::FLOAT8
		FROM (
		    SELECT uid, SUM(credits_delta) AS total_credits
		    FROM   ledger_entries
		    WHERE  created_at_utc >= $1
		      AND  created_at_utc <= $2
		    GROUP  BY uid
		) sub`,
		windowStart, windowEnd,
	).Scan(&p99Credits)
	return userCredits, p99Credits, err
}
