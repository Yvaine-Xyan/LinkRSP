package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type auditEventItem struct {
	EventID      string          `json:"event_id"`
	SchemaVersion string         `json:"schema_version"`
	RuleID       string          `json:"rule_id"`
	SubjectType  string          `json:"subject_type"`
	SubjectID    string          `json:"subject_id"`
	Trigger      string          `json:"trigger"`
	TimestampUTC string          `json:"timestamp_utc"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAtUTC string          `json:"created_at_utc"`
}

type listAuditEventsResponse struct {
	Events []auditEventItem `json:"events"`
	Total  int              `json:"total"`
}

func (s *Server) listAuditEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	subjectType := q.Get("subject_type")
	subjectID := q.Get("subject_id")
	ruleID := q.Get("rule_id")

	limit := 50
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	offset := 0
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	// Build query with optional filters.
	args := []any{}
	where := "WHERE 1=1"
	n := 1
	addFilter := func(col, val string) {
		if val != "" {
			where += " AND " + col + " = $" + strconv.Itoa(n)
			args = append(args, val)
			n++
		}
	}
	addFilter("subject_type", subjectType)
	addFilter("subject_id", subjectID)
	addFilter("rule_id", ruleID)

	// Count total.
	var total int
	err := s.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM audit_events "+where, args...,
	).Scan(&total)
	if err != nil {
		s.Logger.Error("count audit events", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	// Fetch page.
	args = append(args, limit, offset)
	rows, err := s.Pool.Query(r.Context(),
		"SELECT event_id::TEXT, schema_version, rule_id, subject_type, subject_id, "+
			"trigger, timestamp_utc, payload, created_at_utc "+
			"FROM audit_events "+where+
			" ORDER BY timestamp_utc DESC "+
			"LIMIT $"+strconv.Itoa(n)+" OFFSET $"+strconv.Itoa(n+1),
		args...,
	)
	if err != nil {
		s.Logger.Error("list audit events", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	events := []auditEventItem{}
	for rows.Next() {
		var item auditEventItem
		var ts, createdAt time.Time
		var payload []byte
		if err := rows.Scan(
			&item.EventID, &item.SchemaVersion, &item.RuleID,
			&item.SubjectType, &item.SubjectID, &item.Trigger,
			&ts, &payload, &createdAt,
		); err != nil {
			s.Logger.Error("scan audit event row", "error", err)
			continue
		}
		item.TimestampUTC = ts.UTC().Format(time.RFC3339)
		item.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
		item.Payload = json.RawMessage(payload)
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		s.Logger.Error("rows error audit events", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	s.writeJSON(w, http.StatusOK, listAuditEventsResponse{Events: events, Total: total})
}

func (s *Server) getAuditEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("event_id")
	if eventID == "" {
		s.errorJSON(w, http.StatusBadRequest, "event_id required")
		return
	}

	var item auditEventItem
	var ts, createdAt time.Time
	var payload []byte
	err := s.Pool.QueryRow(r.Context(), `
		SELECT event_id::TEXT, schema_version, rule_id, subject_type, subject_id,
		       trigger, timestamp_utc, payload, created_at_utc
		FROM   audit_events
		WHERE  event_id = $1::UUID`,
		eventID,
	).Scan(
		&item.EventID, &item.SchemaVersion, &item.RuleID,
		&item.SubjectType, &item.SubjectID, &item.Trigger,
		&ts, &payload, &createdAt,
	)
	if err != nil {
		if isNotFound(err) {
			s.errorJSON(w, http.StatusNotFound, "audit event not found")
		} else {
			s.Logger.Error("get audit event", "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
		}
		return
	}
	item.TimestampUTC = ts.UTC().Format(time.RFC3339)
	item.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
	item.Payload = json.RawMessage(payload)
	s.writeJSON(w, http.StatusOK, item)
}
