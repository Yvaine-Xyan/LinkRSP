package api

import (
	"net/http"
	"strconv"
	"time"
)

type lrsLedgerQueryResponse struct {
	UID          string             `json:"uid"`
	WindowStart  string             `json:"window_start_utc"`
	WindowEnd    string             `json:"window_end_utc"`
	Entries      []ledgerEntryItem  `json:"entries"`
	TotalCredits float64            `json:"total_credits"`
}

type ledgerEntryItem struct {
	EntryID        string  `json:"entry_id"`
	TaskID         string  `json:"task_id"`
	UID            string  `json:"uid"`
	CreditsDelta   float64 `json:"credits_delta"`
	CreatedAtUTC   string  `json:"created_at_utc"`
	IdempotencyKey string  `json:"idempotency_key"`
}

type attestationIndexQueryResponse struct {
	UID         string                `json:"uid"`
	WindowStart string                `json:"window_start_utc"`
	WindowEnd   string                `json:"window_end_utc"`
	Items       []attestationIndexItem `json:"items"`
}

type attestationIndexItem struct {
	AttestationID     string   `json:"attestation_id"`
	TaskID            string   `json:"task_id"`
	VerificationLevel int      `json:"verification_level"`
	TimestampUTC      string   `json:"timestamp_utc"`
	LocationHash      *string  `json:"location_hash,omitempty"`
	EvidenceRef       *string  `json:"evidence_ref,omitempty"`
}

func (s *Server) lrsLedgerQuery(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		s.errorJSON(w, http.StatusBadRequest, "uid is required")
		return
	}

	windowStart, windowEnd, limit, ok := s.parseInternalWindowQuery(w, r)
	if !ok {
		return
	}

	rows, err := s.Pool.Query(r.Context(), `
		SELECT entry_id::TEXT, task_id::TEXT, uid, credits_delta::FLOAT8, created_at_utc, idempotency_key
		FROM   ledger_entries
		WHERE  uid = $1
		  AND  created_at_utc >= $2
		  AND  created_at_utc <= $3
		ORDER  BY created_at_utc ASC
		LIMIT  $4`, uid, windowStart, windowEnd, limit)
	if err != nil {
		s.Logger.Error("lrs ledger query", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	resp := lrsLedgerQueryResponse{
		UID:         uid,
		WindowStart: windowStart.UTC().Format(time.RFC3339),
		WindowEnd:   windowEnd.UTC().Format(time.RFC3339),
		Entries:     []ledgerEntryItem{},
	}
	for rows.Next() {
		var item ledgerEntryItem
		var createdAt time.Time
		if err := rows.Scan(&item.EntryID, &item.TaskID, &item.UID, &item.CreditsDelta, &createdAt, &item.IdempotencyKey); err != nil {
			s.Logger.Error("scan lrs ledger row", "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
		item.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
		resp.TotalCredits += item.CreditsDelta
		resp.Entries = append(resp.Entries, item)
	}
	if err := rows.Err(); err != nil {
		s.Logger.Error("rows error lrs ledger query", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) attestationIndexQuery(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		s.errorJSON(w, http.StatusBadRequest, "uid is required")
		return
	}

	windowStart, windowEnd, limit, ok := s.parseInternalWindowQuery(w, r)
	if !ok {
		return
	}

	rows, err := s.Pool.Query(r.Context(), `
		SELECT a.attestation_id::TEXT, a.task_id::TEXT, a.verification_level, a.timestamp_utc,
		       a.location_hash, a.evidence_ref
		FROM   attestations a
		JOIN   tasks t ON t.task_id = a.task_id
		WHERE  t.uid_submitter = $1
		  AND  a.timestamp_utc >= $2
		  AND  a.timestamp_utc <= $3
		ORDER  BY a.timestamp_utc ASC
		LIMIT  $4`, uid, windowStart, windowEnd, limit)
	if err != nil {
		s.Logger.Error("attestation index query", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	resp := attestationIndexQueryResponse{
		UID:         uid,
		WindowStart: windowStart.UTC().Format(time.RFC3339),
		WindowEnd:   windowEnd.UTC().Format(time.RFC3339),
		Items:       []attestationIndexItem{},
	}
	for rows.Next() {
		var item attestationIndexItem
		var timestamp time.Time
		if err := rows.Scan(&item.AttestationID, &item.TaskID, &item.VerificationLevel, &timestamp, &item.LocationHash, &item.EvidenceRef); err != nil {
			s.Logger.Error("scan attestation index row", "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
		item.TimestampUTC = timestamp.UTC().Format(time.RFC3339)
		resp.Items = append(resp.Items, item)
	}
	if err := rows.Err(); err != nil {
		s.Logger.Error("rows error attestation index query", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) parseInternalWindowQuery(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, int, bool) {
	windowStartRaw := r.URL.Query().Get("window_start_utc")
	windowEndRaw := r.URL.Query().Get("window_end_utc")
	if windowStartRaw == "" || windowEndRaw == "" {
		s.errorJSON(w, http.StatusBadRequest, "window_start_utc and window_end_utc are required")
		return time.Time{}, time.Time{}, 0, false
	}

	windowStart, err := time.Parse(time.RFC3339, windowStartRaw)
	if err != nil {
		s.errorJSON(w, http.StatusBadRequest, "window_start_utc must be RFC3339")
		return time.Time{}, time.Time{}, 0, false
	}
	windowEnd, err := time.Parse(time.RFC3339, windowEndRaw)
	if err != nil {
		s.errorJSON(w, http.StatusBadRequest, "window_end_utc must be RFC3339")
		return time.Time{}, time.Time{}, 0, false
	}
	windowStart = windowStart.UTC()
	windowEnd = windowEnd.UTC()
	if windowEnd.Before(windowStart) {
		s.errorJSON(w, http.StatusBadRequest, "window_end_utc must be >= window_start_utc")
		return time.Time{}, time.Time{}, 0, false
	}

	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 1000 {
			s.errorJSON(w, http.StatusBadRequest, "limit must be an integer between 1 and 1000")
			return time.Time{}, time.Time{}, 0, false
		}
		limit = parsed
	}
	return windowStart, windowEnd, limit, true
}
