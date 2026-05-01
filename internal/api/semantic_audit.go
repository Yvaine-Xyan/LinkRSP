package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type semanticAuditJobItem struct {
	JobID          string   `json:"job_id"`
	RuleID         string   `json:"rule_id"`
	SubjectType    string   `json:"subject_type"`
	SubjectID      string   `json:"subject_id"`
	TextRef        *string  `json:"text_ref,omitempty"`
	Status         string   `json:"status"`
	Verdict        *string  `json:"verdict,omitempty"`
	Confidence     *float64 `json:"confidence,omitempty"`
	TriggerWords   []string `json:"trigger_words,omitempty"`
	RoutedTo       *string  `json:"routed_to,omitempty"`
	IdempotencyKey string   `json:"idempotency_key"`
	CreatedAtUTC   string   `json:"created_at_utc"`
	UpdatedAtUTC   string   `json:"updated_at_utc"`
}

type listSemanticAuditJobsResponse struct {
	Jobs  []semanticAuditJobItem `json:"jobs"`
	Total int                    `json:"total"`
}

type patchSemanticAuditJobRequest struct {
	Status     *string  `json:"status"`
	Verdict    *string  `json:"verdict"`
	Confidence *float64 `json:"confidence"`
}

var validJobStatuses = map[string]bool{
	"pending": true, "processing": true,
	"human_review": true, "done": true, "skipped": true,
}

var validVerdicts = map[string]bool{
	"PASS": true, "BLOCK": true, "SKIP": true,
}

// listSemanticAuditJobs handles GET /api/v1/internal/semantic-audit-jobs
// Optional query params: status (default "pending"), rule_id, limit (default 50, max 200).
func (s *Server) listSemanticAuditJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	if !validJobStatuses[status] {
		s.errorJSON(w, http.StatusBadRequest, "invalid status value")
		return
	}

	// NULL means "no rule_id filter"; $2::TEXT IS NULL OR rule_id = $2 handles both cases.
	var ruleIDFilter *string
	if raw := r.URL.Query().Get("rule_id"); raw != "" {
		ruleIDFilter = &raw
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			s.errorJSON(w, http.StatusBadRequest, "limit must be an integer between 1 and 200")
			return
		}
		limit = parsed
	}

	rows, err := s.Pool.Query(r.Context(), `
		SELECT job_id::TEXT, rule_id, subject_type, subject_id, text_ref,
		       status, verdict, confidence::FLOAT8, trigger_words, routed_to,
		       idempotency_key, created_at_utc, updated_at_utc
		FROM   semantic_audit_jobs
		WHERE  status = $1
		  AND  ($2::TEXT IS NULL OR rule_id = $2)
		ORDER  BY created_at_utc ASC
		LIMIT  $3`,
		status, ruleIDFilter, limit,
	)
	if err != nil {
		s.Logger.Error("list semantic audit jobs", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	resp := listSemanticAuditJobsResponse{Jobs: []semanticAuditJobItem{}}
	for rows.Next() {
		item, scanErr := scanSemanticAuditJobRow(rows)
		if scanErr != nil {
			s.Logger.Error("scan semantic audit job row", "error", scanErr)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
		resp.Jobs = append(resp.Jobs, item)
		resp.Total++
	}
	if err := rows.Err(); err != nil {
		s.Logger.Error("rows error list semantic audit jobs", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// patchSemanticAuditJob handles PATCH /api/v1/internal/semantic-audit-jobs/{job_id}
// Updates status, verdict, and/or confidence on a single job (human review action).
// Only semantic_audit_jobs is mutable (queue table); ledger/audit tables remain append-only (C-6).
func (s *Server) patchSemanticAuditJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	if _, err := uuid.Parse(jobID); err != nil {
		s.errorJSON(w, http.StatusBadRequest, "invalid job_id: must be UUID")
		return
	}

	var req patchSemanticAuditJobRequest
	if err := s.decodeBody(r, &req); err != nil {
		s.errorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status == nil && req.Verdict == nil && req.Confidence == nil {
		s.errorJSON(w, http.StatusBadRequest, "at least one of status, verdict, confidence is required")
		return
	}
	if req.Status != nil && !validJobStatuses[*req.Status] {
		s.errorJSON(w, http.StatusBadRequest, "invalid status value")
		return
	}
	if req.Verdict != nil && !validVerdicts[*req.Verdict] {
		s.errorJSON(w, http.StatusBadRequest, "invalid verdict value")
		return
	}
	if req.Confidence != nil && (*req.Confidence < 0 || *req.Confidence > 1) {
		s.errorJSON(w, http.StatusBadRequest, "confidence must be between 0 and 1")
		return
	}

	// Use a transaction so the human write-back and its audit-event record are atomic.
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.Logger.Error("begin patch semantic audit job", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	result, err := tx.Exec(r.Context(), `
			UPDATE semantic_audit_jobs
			SET    status         = COALESCE($1, status),
			       verdict        = COALESCE($2, verdict),
			       confidence     = COALESCE($3, confidence),
			       updated_at_utc = NOW()
			WHERE  job_id = $4::UUID`,
		req.Status, req.Verdict, req.Confidence, jobID,
	)
	if err != nil {
		s.Logger.Error("patch semantic audit job", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	if result.RowsAffected() == 0 {
		s.errorJSON(w, http.StatusNotFound, "job not found")
		return
	}

	row := tx.QueryRow(r.Context(), `
			SELECT job_id::TEXT, rule_id, subject_type, subject_id, text_ref,
			       status, verdict, confidence::FLOAT8, trigger_words, routed_to,
			       idempotency_key, created_at_utc, updated_at_utc
			FROM   semantic_audit_jobs
			WHERE  job_id = $1::UUID`, jobID,
	)
	item, err := scanSemanticAuditJobRow(row)
	if err != nil {
		s.Logger.Error("fetch updated semantic audit job", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	// Store an audit event for the write-back. This is intentionally job-scoped so it is
	// replayable regardless of the subject type (task / ipo_application / etc.).
	if err := s.storeSemanticWritebackAuditEvent(r, tx, item); err != nil {
		s.Logger.Error("store semantic writeback audit event", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := tx.Commit(r.Context()); err != nil && err != pgx.ErrTxClosed {
		s.Logger.Error("commit patch semantic audit job", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	s.writeJSON(w, http.StatusOK, item)
}

func (s *Server) storeSemanticWritebackAuditEvent(r *http.Request, tx pgx.Tx, job semanticAuditJobItem) error {
	// If the patch did not result in a verdict, we still record a PASS write-back event
	// only when the job has reached a terminal state (done/skipped). This keeps noise low
	// while preserving replayability for actual decisions.
	terminal := job.Status == "done" || job.Status == "skipped"
	if job.Verdict == nil && !terminal {
		return nil
	}

	category := "semantic_audit"
	trigger := "periodic_audit"

	// Map queue verdicts to audit-event verdicts. SKIP means "no decision needed";
	// we store it as PASS with a matched_pattern note.
	verdict := audit.VerdictPass
	severity := "warn"
	matched := fmt.Sprintf("semantic write-back: status=%s", job.Status)
	if job.Verdict != nil {
		matched = fmt.Sprintf("semantic write-back: status=%s verdict=%s", job.Status, *job.Verdict)
		switch *job.Verdict {
		case "PASS":
			verdict = audit.VerdictPass
			severity = "warn"
		case "BLOCK":
			verdict = audit.VerdictBlock
			severity = "block"
		case "SKIP":
			verdict = audit.VerdictPass
			severity = "warn"
		}
	}

	builder := audit.NewBuilder(job.RuleID, category, trigger).WithSeverity(severity)
	var ev audit.Event
	switch verdict {
	case audit.VerdictBlock:
		ev = builder.BuildBlock("semantic_audit_job", job.JobID, nil, matched, []string{"review"})
	default:
		ev = builder.BuildPass("semantic_audit_job", job.JobID)
		ev.Evidence.MatchedPattern = &matched
	}

	// Preserve linkage for downstream replay: secondary_id points back to the original subject.
	ev.Subject.SecondaryID = &job.SubjectID

	// For semantic audits, confidence is meaningful and should be stored when present.
	if job.Confidence != nil {
		ev.Verdict.Confidence = job.Confidence
	}

	return audit.StoreWithExecer(r.Context(), tx, ev)
}

// scanner abstracts pgx.Rows and pgx.Row so scanSemanticAuditJobRow can serve both.
type scanner interface {
	Scan(...any) error
}

func scanSemanticAuditJobRow(s scanner) (semanticAuditJobItem, error) {
	var item semanticAuditJobItem
	var createdAt, updatedAt time.Time
	var triggerWords []string
	err := s.Scan(
		&item.JobID, &item.RuleID, &item.SubjectType, &item.SubjectID, &item.TextRef,
		&item.Status, &item.Verdict, &item.Confidence, &triggerWords, &item.RoutedTo,
		&item.IdempotencyKey, &createdAt, &updatedAt,
	)
	if err != nil {
		return semanticAuditJobItem{}, err
	}
	item.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
	item.UpdatedAtUTC = updatedAt.UTC().Format(time.RFC3339)
	if len(triggerWords) > 0 {
		item.TriggerWords = triggerWords
	}
	return item, nil
}
