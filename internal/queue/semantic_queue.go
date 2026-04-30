package queue

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type JobStatus string
type Verdict string
type RoutedTo string

const (
	StatusPending     JobStatus = "pending"
	StatusProcessing  JobStatus = "processing"
	StatusHumanReview JobStatus = "human_review"
	StatusDone        JobStatus = "done"
	StatusSkipped     JobStatus = "skipped"

	VerdictPass  Verdict = "PASS"
	VerdictBlock Verdict = "BLOCK"
	VerdictSkip  Verdict = "SKIP"

	RouteHumanReview RoutedTo = "human_review_queue"
	RouteLLM         RoutedTo = "llm_queue"
)

// Job is the input descriptor for a new semantic audit job.
type Job struct {
	RuleID      string
	SubjectType string
	SubjectID   string
	TextRef     string
	RoutedTo    RoutedTo
}

// Enqueue inserts a semantic audit job into semantic_audit_jobs.
// Idempotent: same (rule_id, subject_type, subject_id) within the same UTC minute
// returns nil without inserting a duplicate.
func Enqueue(ctx context.Context, pool *pgxpool.Pool, job Job, triggerWords []string) error {
	ikey := idempotencyKey(job.RuleID, job.SubjectType, job.SubjectID, time.Now().UTC())
	_, err := pool.Exec(ctx, `
		INSERT INTO semantic_audit_jobs
			(rule_id, subject_type, subject_id, text_ref, trigger_words, routed_to, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		job.RuleID, job.SubjectType, job.SubjectID,
		nullableText(job.TextRef),
		triggerWords,
		nullableText(string(job.RoutedTo)),
		ikey,
	)
	return err
}

// PreFilter checks text against a list of trigger words (case-insensitive substring match).
// Returns the subset of triggerWords found in text. An empty result means no routing needed.
func PreFilter(text string, triggerWords []string) []string {
	lower := strings.ToLower(text)
	var matched []string
	for _, w := range triggerWords {
		if strings.Contains(lower, strings.ToLower(w)) {
			matched = append(matched, w)
		}
	}
	return matched
}

// Route returns the appropriate RoutedTo value for a Phase D job.
// Phase E will extend this to route high-confidence cases to llm_queue.
func Route(_ []string) RoutedTo {
	return RouteHumanReview
}

func idempotencyKey(ruleID, subjectType, subjectID string, t time.Time) string {
	minute := t.Truncate(time.Minute).Format(time.RFC3339)
	raw := fmt.Sprintf("%s|%s|%s|%s", ruleID, subjectType, subjectID, minute)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:16])
}

func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}
