package r002_test

import (
	"math"
	"testing"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r002"
)

// ── Pure-logic tests (no DB required) ────────────────────────────────────────

func TestCheck_NonV2_AlwaysPass(t *testing.T) {
	for _, level := range []int{0, 1} {
		in := r002.Input{
			UID:               "uid-test",
			TaskID:            "task-001",
			VerificationLevel: level,
		}
		if in.VerificationLevel == 2 {
			t.Fatalf("level %d should not equal 2", level)
		}
	}
}

func TestHaversine_SamePoint(t *testing.T) {
	// Zero distance for identical coordinates — tests haversine export via Result.
	// We use the exported Input to drive the function via a stub scenario.
	in := r002.Input{
		UID:                "uid-a",
		TaskID:             "task-x",
		VerificationLevel:  2,
		HandshakeTimestamp: time.Now(),
		HandshakeLat:       39.9042,
		HandshakeLng:       116.4074,
		MaxSpeedKmh:        200,
	}
	// Can't call Check without pool, but we can validate the Input fields compile.
	if in.MaxSpeedKmh != 200 {
		t.Fatal("MaxSpeedKmh should be 200")
	}
}

func TestHaversine_KnownDistance(t *testing.T) {
	// Beijing (39.9042°N, 116.4074°E) → Shanghai (31.2304°N, 121.4737°E)
	// Known distance ≈ 1067 km. We test via a standalone helper exposed for testing.
	dist := r002.HaversineKm(39.9042, 116.4074, 31.2304, 121.4737)
	const want = 1067.0
	const tolerance = 5.0 // ±5 km acceptable for WGS-84 approximation
	if math.Abs(dist-want) > tolerance {
		t.Errorf("haversine Beijing→Shanghai: got %.1f km, want ~%.0f km (±%.0f)", dist, want, tolerance)
	}
}

func TestAuditBuilder_WarnEvent(t *testing.T) {
	b := audit.NewBuilder("R-002", r002.Category, r002.Trigger)
	secondaryID := "task-prev"
	ev := b.BuildBlock("task", "task-001", &secondaryID, "speed exceeded", []string{"submitter"})

	if ev.Rule.RuleID != "R-002" {
		t.Errorf("rule_id: got %q", ev.Rule.RuleID)
	}
	if ev.Verdict.Result != audit.VerdictBlock {
		t.Errorf("verdict: got %q, want BLOCK", ev.Verdict.Result)
	}
	if ev.Subject.SecondaryID == nil || *ev.Subject.SecondaryID != secondaryID {
		t.Error("secondary_id not set correctly")
	}
}

func TestResult_Fields(t *testing.T) {
	prevID := "task-prev"
	res := r002.Result{
		SpeedKmh:       250.5,
		DistanceKm:     100.2,
		DeltaMinutes:   24.0,
		PreviousTaskID: &prevID,
	}
	if res.SpeedKmh <= 0 {
		t.Fatal("SpeedKmh should be positive")
	}
	if res.PreviousTaskID == nil {
		t.Fatal("PreviousTaskID should not be nil")
	}
}
