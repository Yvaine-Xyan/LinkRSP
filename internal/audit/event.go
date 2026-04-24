package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Verdict string

const (
	VerdictPass  Verdict = "PASS"
	VerdictFlag  Verdict = "FLAG"
	VerdictBlock Verdict = "BLOCK"
)

type Event struct {
	EventID       string    `json:"event_id"`
	SchemaVersion string    `json:"schema_version"`
	Rule          RuleRef   `json:"rule"`
	Scope         Scope     `json:"scope"`
	Subject       Subject   `json:"subject"`
	Verdict       VerdictOf `json:"verdict"`
	Evidence      Evidence  `json:"evidence"`
	Trace         Trace     `json:"trace"`
	ActionTaken   Action    `json:"action_taken"`
}

type RuleRef struct {
	RuleID    string `json:"rule_id"`
	Version   string `json:"rule_version"`
	Category  string `json:"category"`
}

type Scope struct {
	Trigger    string `json:"trigger"`
	AuditScope string `json:"audit_scope"`
}

type Subject struct {
	Type        string  `json:"type"`
	ID          string  `json:"id"`
	SecondaryID *string `json:"secondary_id,omitempty"`
}

type VerdictOf struct {
	Result      Verdict  `json:"result"`
	Severity    string   `json:"severity"`
	Confidence  *float64 `json:"confidence"`
	AutoActioned bool    `json:"auto_actioned"`
}

type Evidence struct {
	MatchedPattern  *string  `json:"matched_pattern,omitempty"`
	ConflictingIDs  []string `json:"conflicting_ids,omitempty"`
}

type Trace struct {
	TraceID      string  `json:"trace_id"`
	TimestampUTC string  `json:"timestamp_utc"`
	TriggeredBy  string  `json:"triggered_by"`
	ReviewerUID  *string `json:"reviewer_uid,omitempty"`
}

type Action struct {
	Notified      []string `json:"notified"`
	RoutedTo      *string  `json:"routed_to,omitempty"`
	AppealEligible bool    `json:"appeal_eligible"`
}

// Builder constructs an audit Event for a single rule execution.
type Builder struct {
	rule     RuleRef
	trigger  string
	traceID  string
}

func NewBuilder(ruleID, category, trigger string) *Builder {
	return &Builder{
		rule:    RuleRef{RuleID: ruleID, Version: "1.0", Category: category},
		trigger: trigger,
		traceID: uuid.New().String(),
	}
}

func (b *Builder) BuildBlock(subjectType, subjectID string, secondaryID *string, matchedPattern string, notified []string) Event {
	return b.build(VerdictBlock, "block", true, subjectType, subjectID, secondaryID, &matchedPattern, notified)
}

func (b *Builder) BuildPass(subjectType, subjectID string) Event {
	return b.build(VerdictPass, "block", false, subjectType, subjectID, nil, nil, nil)
}

func (b *Builder) build(verdict Verdict, severity string, autoActioned bool,
	subjectType, subjectID string, secondaryID *string,
	matchedPattern *string, notified []string) Event {

	now := time.Now().UTC()
	if notified == nil {
		notified = []string{}
	}
	return Event{
		EventID:       uuid.New().String(),
		SchemaVersion: "1.0",
		Rule:          b.rule,
		Scope:         Scope{Trigger: b.trigger, AuditScope: "task"},
		Subject:       Subject{Type: subjectType, ID: subjectID, SecondaryID: secondaryID},
		Verdict:       VerdictOf{Result: verdict, Severity: severity, Confidence: nil, AutoActioned: autoActioned},
		Evidence:      Evidence{MatchedPattern: matchedPattern},
		Trace:         Trace{TraceID: b.traceID, TimestampUTC: now.Format(time.RFC3339), TriggeredBy: "system_auto"},
		ActionTaken:   Action{Notified: notified, AppealEligible: verdict == VerdictBlock},
	}
}

// IdempotencyKey returns the dedup key per audit-event-schema-v1.0.md §2.
func IdempotencyKey(ruleID, subjectType, subjectID, trigger string, t time.Time) string {
	minute := t.UTC().Truncate(time.Minute).Format("2006-01-02T15:04")
	return fmt.Sprintf("%s|%s|%s|%s|%s", ruleID, subjectType, subjectID, trigger, minute)
}

// Store persists an audit event to the append-only audit_events table (C-6).
func Store(ctx context.Context, pool *pgxpool.Pool, ev Event) error {
	ikey := IdempotencyKey(
		ev.Rule.RuleID, ev.Subject.Type, ev.Subject.ID,
		ev.Scope.Trigger, time.Now().UTC(),
	)
	_, err := pool.Exec(ctx, `
		INSERT INTO audit_events
			(event_id, schema_version, payload, rule_id, subject_type, subject_id,
			 trigger, timestamp_utc, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		ev.EventID, ev.SchemaVersion, ev, ev.Rule.RuleID,
		ev.Subject.Type, ev.Subject.ID, ev.Scope.Trigger,
		ev.Trace.TimestampUTC, ikey,
	)
	return err
}
