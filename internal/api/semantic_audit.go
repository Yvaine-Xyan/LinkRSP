package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/YvaineHe/linkrsp/internal/audit"
	"github.com/YvaineHe/linkrsp/internal/queue"
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

type enqueueSemanticAuditJobRequest struct {
	RuleID       string   `json:"rule_id"`
	SubjectType  string   `json:"subject_type"`
	SubjectID    string   `json:"subject_id"`
	TextRef      *string  `json:"text_ref,omitempty"`
	Text         *string  `json:"text,omitempty"`
	TriggerWords []string `json:"trigger_words,omitempty"`
}

type semanticAuditJobStatsResponse struct {
	CountsByStatus map[string]int `json:"counts_by_status"`
	OldestPending  *string        `json:"oldest_pending_utc,omitempty"`
	OldestHumanRev *string        `json:"oldest_human_review_utc,omitempty"`
}

type semanticAuditJobReplayResponse struct {
	Job            semanticAuditJobItem `json:"job"`
	WriteBackAudit []auditEventItem     `json:"writeback_audit_events"`
	Subject        any                  `json:"subject,omitempty"`
	SubjectAudit   []auditEventItem     `json:"subject_audit_events,omitempty"`
	LedgerEntries  []any                `json:"ledger_entries,omitempty"`
	Attestations   []any                `json:"attestations,omitempty"`
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

// enqueueSemanticAuditJob handles POST /api/v1/internal/semantic-audit-jobs/enqueue
// It is an internal ingestion point used by operators or community-run tooling.
// The system does not prescribe community SOP; it only provides an auditable queue path.
func (s *Server) enqueueSemanticAuditJob(w http.ResponseWriter, r *http.Request) {
	if !s.consumeSemanticEnqueueToken() {
		s.errorJSON(w, http.StatusTooManyRequests, "semantic audit enqueue rate limit exceeded")
		return
	}

	var req enqueueSemanticAuditJobRequest
	if err := s.decodeBody(r, &req); err != nil {
		s.errorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RuleID == "" || req.SubjectType == "" || req.SubjectID == "" {
		s.errorJSON(w, http.StatusBadRequest, "rule_id, subject_type, subject_id are required")
		return
	}
	if req.TextRef == nil && req.Text == nil {
		s.errorJSON(w, http.StatusBadRequest, "at least one of text_ref or text is required")
		return
	}

	// Pre-filter is optional: caller may pass trigger_words, or rely on routing defaults.
	var matched []string
	if req.Text != nil && len(req.TriggerWords) > 0 {
		matched = queue.PreFilter(*req.Text, req.TriggerWords)
	} else if len(req.TriggerWords) > 0 {
		// Without raw text we treat supplied trigger_words as "matched".
		matched = req.TriggerWords
	}

	routed := queue.Route(matched)
	textRef := ""
	if req.TextRef != nil {
		textRef = *req.TextRef
	} else if req.Text != nil {
		// Store the raw text as text_ref for Phase D minimalism; Phase E should move this
		// to an object store reference or hashed pointer.
		textRef = *req.Text
	}

	if err := queue.Enqueue(r.Context(), s.Pool, queue.Job{
		RuleID:      req.RuleID,
		SubjectType: req.SubjectType,
		SubjectID:   req.SubjectID,
		TextRef:     textRef,
		RoutedTo:    routed,
	}, matched); err != nil {
		s.Logger.Error("enqueue semantic audit job", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	s.writeJSON(w, http.StatusAccepted, map[string]any{
		"result":        "accepted",
		"rule_id":       req.RuleID,
		"subject_type":  req.SubjectType,
		"subject_id":    req.SubjectID,
		"routed_to":     string(routed),
		"trigger_words": matched,
	})
}

// semanticAuditJobStats handles GET /api/v1/internal/semantic-audit-jobs/stats
// It provides minimal queue observability for Phase D operations.
func (s *Server) semanticAuditJobStats(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Pool.Query(r.Context(), `
		SELECT status, COUNT(*)::INT
		FROM semantic_audit_jobs
		GROUP BY status`)
	if err != nil {
		s.Logger.Error("semantic audit job stats", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			s.Logger.Error("scan semantic audit job stats", "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
		counts[status] = count
	}
	if err := rows.Err(); err != nil {
		s.Logger.Error("rows error semantic audit job stats", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	var oldestPending *time.Time
	_ = s.Pool.QueryRow(r.Context(), `
		SELECT MIN(created_at_utc) FROM semantic_audit_jobs WHERE status='pending'`).Scan(&oldestPending)
	var oldestHuman *time.Time
	_ = s.Pool.QueryRow(r.Context(), `
		SELECT MIN(created_at_utc) FROM semantic_audit_jobs WHERE status='human_review'`).Scan(&oldestHuman)

	resp := semanticAuditJobStatsResponse{CountsByStatus: counts}
	if oldestPending != nil {
		v := oldestPending.UTC().Format(time.RFC3339)
		resp.OldestPending = &v
	}
	if oldestHuman != nil {
		v := oldestHuman.UTC().Format(time.RFC3339)
		resp.OldestHumanRev = &v
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// semanticAuditJobReplay handles GET /api/v1/internal/semantic-audit-jobs/{job_id}/replay
// It returns an evidence bundle for human/ops replay: the job, its write-back audit events,
// and (when subject_type=task) the linked task/audit/attestations/ledger.
func (s *Server) semanticAuditJobReplay(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	if _, err := uuid.Parse(jobID); err != nil {
		s.errorJSON(w, http.StatusBadRequest, "invalid job_id: must be UUID")
		return
	}

	row := s.Pool.QueryRow(r.Context(), `
		SELECT job_id::TEXT, rule_id, subject_type, subject_id, text_ref,
		       status, verdict, confidence::FLOAT8, trigger_words, routed_to,
		       idempotency_key, created_at_utc, updated_at_utc
		FROM semantic_audit_jobs
		WHERE job_id = $1::UUID`, jobID)
	job, err := scanSemanticAuditJobRow(row)
	if err != nil {
		if isNotFound(err) {
			s.errorJSON(w, http.StatusNotFound, "job not found")
			return
		}
		s.Logger.Error("replay fetch job", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	writebackAudit, err := s.fetchAuditEvents(r, "semantic_audit_job", job.JobID, job.RuleID, 200)
	if err != nil {
		s.Logger.Error("replay fetch writeback audit", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	resp := semanticAuditJobReplayResponse{
		Job:            job,
		WriteBackAudit: writebackAudit,
	}

	// For now we only expand task subjects; other subject types remain opaque but replayable
	// via the job record + writeback audit events.
	if job.SubjectType != "task" {
		s.writeJSON(w, http.StatusOK, resp)
		return
	}

	// Task snapshot.
	var task struct {
		TaskID          string  `json:"task_id"`
		UIDSubmitter    string  `json:"uid_submitter"`
		CommunityID     *string `json:"community_id,omitempty"`
		DescriptionText string  `json:"description_text"`
		StartTimeUTC    string  `json:"start_time_utc"`
		EndTimeUTC      string  `json:"end_time_utc"`
		LocationHash    *string `json:"location_hash,omitempty"`
		CreatedAtUTC    string  `json:"created_at_utc"`
	}
	var start, end, createdAt time.Time
	err = s.Pool.QueryRow(r.Context(), `
		SELECT task_id::TEXT, uid_submitter, community_id::TEXT,
		       description_text, start_time_utc, end_time_utc,
		       location_hash, created_at_utc
		FROM tasks
		WHERE task_id = $1::UUID`, job.SubjectID,
	).Scan(
		&task.TaskID, &task.UIDSubmitter, &task.CommunityID,
		&task.DescriptionText, &start, &end,
		&task.LocationHash, &createdAt,
	)
	if err == nil {
		task.StartTimeUTC = start.UTC().Format(time.RFC3339)
		task.EndTimeUTC = end.UTC().Format(time.RFC3339)
		task.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
		resp.Subject = task
	}

	subjectAudit, err := s.fetchAuditEvents(r, "task", job.SubjectID, "", 200)
	if err != nil {
		s.Logger.Error("replay fetch subject audit", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	resp.SubjectAudit = subjectAudit

	// Ledger entries for the task (if any).
	ledgerRows, err := s.Pool.Query(r.Context(), `
		SELECT entry_id::TEXT, task_id::TEXT, uid, credits_delta::FLOAT8, created_at_utc, idempotency_key
		FROM ledger_entries
		WHERE task_id = $1::UUID
		ORDER BY created_at_utc ASC`, job.SubjectID)
	if err != nil {
		s.Logger.Error("replay fetch ledger entries", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer ledgerRows.Close()
	for ledgerRows.Next() {
		var item struct {
			EntryID        string    `json:"entry_id"`
			TaskID         string    `json:"task_id"`
			UID            string    `json:"uid"`
			CreditsDelta   float64   `json:"credits_delta"`
			CreatedAtUTC   string    `json:"created_at_utc"`
			IdempotencyKey string    `json:"idempotency_key"`
			CreatedAtRaw   time.Time `json:"-"`
		}
		if err := ledgerRows.Scan(&item.EntryID, &item.TaskID, &item.UID, &item.CreditsDelta, &item.CreatedAtRaw, &item.IdempotencyKey); err != nil {
			s.Logger.Error("replay scan ledger entry", "job_id", jobID, "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
		item.CreatedAtUTC = item.CreatedAtRaw.UTC().Format(time.RFC3339)
		resp.LedgerEntries = append(resp.LedgerEntries, item)
	}
	if err := ledgerRows.Err(); err != nil {
		s.Logger.Error("replay rows ledger entry", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	// Attestations for the task.
	attRows, err := s.Pool.Query(r.Context(), `
		SELECT attestation_id::TEXT, task_id::TEXT, verification_level,
		       timestamp_utc, location_hash, evidence_ref, created_at_utc
		FROM attestations
		WHERE task_id = $1::UUID
		ORDER BY created_at_utc ASC`, job.SubjectID)
	if err != nil {
		s.Logger.Error("replay fetch attestations", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer attRows.Close()
	for attRows.Next() {
		var item struct {
			AttestationID     string    `json:"attestation_id"`
			TaskID            string    `json:"task_id"`
			VerificationLevel int       `json:"verification_level"`
			TimestampUTC      string    `json:"timestamp_utc"`
			LocationHash      *string   `json:"location_hash,omitempty"`
			EvidenceRef       *string   `json:"evidence_ref,omitempty"`
			CreatedAtUTC      string    `json:"created_at_utc"`
			TimestampRaw      time.Time `json:"-"`
			CreatedAtRaw      time.Time `json:"-"`
		}
		if err := attRows.Scan(
			&item.AttestationID, &item.TaskID, &item.VerificationLevel,
			&item.TimestampRaw, &item.LocationHash, &item.EvidenceRef, &item.CreatedAtRaw,
		); err != nil {
			s.Logger.Error("replay scan attestation", "job_id", jobID, "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
		item.TimestampUTC = item.TimestampRaw.UTC().Format(time.RFC3339)
		item.CreatedAtUTC = item.CreatedAtRaw.UTC().Format(time.RFC3339)
		resp.Attestations = append(resp.Attestations, item)
	}
	if err := attRows.Err(); err != nil {
		s.Logger.Error("replay rows attestation", "job_id", jobID, "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) fetchAuditEvents(r *http.Request, subjectType, subjectID, ruleID string, limit int) ([]auditEventItem, error) {
	args := []any{}
	where := "WHERE subject_type = $1 AND subject_id = $2"
	args = append(args, subjectType, subjectID)
	n := 3
	if ruleID != "" {
		where += " AND rule_id = $" + strconv.Itoa(n)
		args = append(args, ruleID)
		n++
	}
	args = append(args, limit)

	rows, err := s.Pool.Query(r.Context(),
		"SELECT event_id::TEXT, schema_version, rule_id, subject_type, subject_id, "+
			"trigger, timestamp_utc, payload, created_at_utc "+
			"FROM audit_events "+where+
			" ORDER BY timestamp_utc DESC "+
			"LIMIT $"+strconv.Itoa(n),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []auditEventItem
	for rows.Next() {
		var item auditEventItem
		var ts, createdAt time.Time
		var payload []byte
		if err := rows.Scan(
			&item.EventID, &item.SchemaVersion, &item.RuleID,
			&item.SubjectType, &item.SubjectID, &item.Trigger,
			&ts, &payload, &createdAt,
		); err != nil {
			return nil, err
		}
		item.TimestampUTC = ts.UTC().Format(time.RFC3339)
		item.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
		item.Payload = payload
		out = append(out, item)
	}
	return out, rows.Err()
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
