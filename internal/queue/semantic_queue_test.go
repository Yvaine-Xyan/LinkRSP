package queue

import (
	"testing"
	"time"
)

func TestPreFilter_NoMatch(t *testing.T) {
	words := []string{"诈骗", "高利贷", "fraud"}
	matched := PreFilter("这是一段正常的任务描述", words)
	if len(matched) != 0 {
		t.Fatalf("expected no match, got %v", matched)
	}
}

func TestPreFilter_SingleMatch(t *testing.T) {
	words := []string{"诈骗", "高利贷", "fraud"}
	matched := PreFilter("请警惕fraud类行为", words)
	if len(matched) != 1 || matched[0] != "fraud" {
		t.Fatalf("expected [fraud], got %v", matched)
	}
}

func TestPreFilter_MultiMatch(t *testing.T) {
	words := []string{"诈骗", "高利贷", "fraud"}
	matched := PreFilter("涉及诈骗和高利贷的描述", words)
	if len(matched) != 2 {
		t.Fatalf("expected 2 matches, got %v", matched)
	}
}

func TestPreFilter_CaseInsensitive(t *testing.T) {
	words := []string{"Fraud"}
	matched := PreFilter("FRAUD activity detected", words)
	if len(matched) != 1 {
		t.Fatalf("expected case-insensitive match, got %v", matched)
	}
}

func TestPreFilter_NilTriggerWords(t *testing.T) {
	matched := PreFilter("text", nil)
	if len(matched) != 0 {
		t.Fatalf("expected no match for nil trigger words, got %v", matched)
	}
}

func TestRoute_PhaseD_AlwaysHumanReview(t *testing.T) {
	route := Route([]string{"诈骗"})
	if route != RouteHumanReview {
		t.Fatalf("expected human_review_queue in Phase D, got %q", route)
	}
}

func TestIdempotencyKey_StableWithinMinute(t *testing.T) {
	t1 := mustParseRFC3339(t, "2026-05-01T10:00:00Z")
	t2 := mustParseRFC3339(t, "2026-05-01T10:00:45Z")
	k1 := idempotencyKey("S-001", "task", "abc", t1)
	k2 := idempotencyKey("S-001", "task", "abc", t2)
	if k1 != k2 {
		t.Fatalf("idempotency key must be stable within same minute: %q vs %q", k1, k2)
	}
}

func TestIdempotencyKey_DiffersAcrossMinutes(t *testing.T) {
	t1 := mustParseRFC3339(t, "2026-05-01T10:00:00Z")
	t2 := mustParseRFC3339(t, "2026-05-01T10:01:00Z")
	k1 := idempotencyKey("S-001", "task", "abc", t1)
	k2 := idempotencyKey("S-001", "task", "abc", t2)
	if k1 == k2 {
		t.Fatal("idempotency key must differ across minutes")
	}
}

func mustParseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("invalid RFC3339 time %q: %v", s, err)
	}
	return ts
}
