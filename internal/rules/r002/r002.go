// Package r002 implements R-002: V=2 Handshake Speed Paradox.
// Two consecutive V=2 (near-field) attestations for the same UID imply
// a physical displacement. If the implied travel speed exceeds the
// configured threshold (default 200 km/h) the settlement is blocked.
package r002

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuleID             = "R-002"
	Category           = "physical_paradox"
	Trigger            = "post_execution"
	DefaultMaxSpeedKmh = 200.0
)

// Input holds the fields checked by R-002 (from R-002.yaml `inputs`).
type Input struct {
	UID                string
	TaskID             string
	VerificationLevel  int
	HandshakeTimestamp time.Time
	HandshakeLat       float64
	HandshakeLng       float64
	MaxSpeedKmh        float64 // 0 → DefaultMaxSpeedKmh (200)
}

// Result is the outcome of a single R-002 evaluation.
type Result struct {
	Event          audit.Event
	SpeedKmh       float64
	DistanceKm     float64
	DeltaMinutes   float64
	PreviousTaskID *string // nil when no previous V=2 exists
}

// Check queries the most recent prior V=2 attestation for the same UID,
// computes implied speed, and blocks if it exceeds MaxSpeedKmh.
func Check(ctx context.Context, pool *pgxpool.Pool, in Input) (Result, error) {
	if in.VerificationLevel != 2 {
		ev := audit.NewBuilder(RuleID, Category, Trigger).BuildPass("task", in.TaskID)
		return Result{Event: ev}, nil
	}

	maxSpeed := in.MaxSpeedKmh
	if maxSpeed == 0 {
		maxSpeed = DefaultMaxSpeedKmh
	}

	prev, err := findPreviousV2(ctx, pool, in)
	if err != nil {
		return Result{}, fmt.Errorf("r002: query previous V=2 attestation: %w", err)
	}

	builder := audit.NewBuilder(RuleID, Category, Trigger)

	if prev == nil {
		ev := builder.BuildPass("task", in.TaskID)
		return Result{Event: ev}, nil
	}

	deltaMinutes := in.HandshakeTimestamp.Sub(prev.Timestamp).Minutes()
	if deltaMinutes <= 0 {
		// Zero or negative gap: timestamps are suspect — PASS conservatively.
		ev := builder.BuildPass("task", in.TaskID)
		return Result{Event: ev, PreviousTaskID: &prev.TaskID}, nil
	}

	distKm := HaversineKm(prev.Lat, prev.Lng, in.HandshakeLat, in.HandshakeLng)
	speedKmh := distKm / (deltaMinutes / 60.0)

	res := Result{
		SpeedKmh:       speedKmh,
		DistanceKm:     distKm,
		DeltaMinutes:   deltaMinutes,
		PreviousTaskID: &prev.TaskID,
	}

	if speedKmh <= maxSpeed {
		res.Event = builder.BuildPass("task", in.TaskID)
		return res, nil
	}

	pattern := fmt.Sprintf(
		"uid %s: %.1f km in %.1f min = %.0f km/h between V=2 tasks %s→%s (limit %.0f km/h)",
		in.UID, distKm, deltaMinutes, speedKmh, prev.TaskID, in.TaskID, maxSpeed,
	)
	ev := builder.BuildBlock("task", in.TaskID, &prev.TaskID, pattern, []string{"submitter"})
	ev.Evidence.ConflictingIDs = []string{prev.TaskID}
	res.Event = ev
	return res, nil
}

// HaversineKm returns the great-circle distance in kilometres between two
// WGS-84 coordinates.
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthKm = 6371.0
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func toRad(deg float64) float64 { return deg * math.Pi / 180 }

type previousHandshake struct {
	TaskID    string
	Timestamp time.Time
	Lat       float64
	Lng       float64
}

// findPreviousV2 returns the most recent V=2 attestation by the same UID
// before the current handshake timestamp, or nil if none exists.
func findPreviousV2(ctx context.Context, pool *pgxpool.Pool, in Input) (*previousHandshake, error) {
	const q = `
		SELECT t.task_id, a.timestamp_utc, a.location_lat, a.location_lng
		FROM   attestations a
		JOIN   tasks t ON t.task_id = a.task_id
		WHERE  t.uid_submitter    = $1
		  AND  a.task_id         != $2
		  AND  a.verification_level = 2
		  AND  a.timestamp_utc   < $3
		  AND  a.location_lat   IS NOT NULL
		  AND  a.location_lng   IS NOT NULL
		ORDER  BY a.timestamp_utc DESC
		LIMIT  1`

	rows, err := pool.Query(ctx, q, in.UID, in.TaskID, in.HandshakeTimestamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}

	var p previousHandshake
	if err := rows.Scan(&p.TaskID, &p.Timestamp, &p.Lat, &p.Lng); err != nil {
		return nil, err
	}
	return &p, nil
}
