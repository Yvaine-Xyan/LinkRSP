package api

import (
	"net/http"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r003"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r004"
	"github.com/google/uuid"
)

type createTaskRequest struct {
	UIDSubmitter    string  `json:"uid_submitter"`
	CommunityID     *string `json:"community_id"`
	DescriptionText string  `json:"description_text"`
	StartTimeUTC    string  `json:"start_time_utc"`
	EndTimeUTC      string  `json:"end_time_utc"`
	LocationHash    *string `json:"location_hash"`
}

type taskResponse struct {
	TaskID          string  `json:"task_id"`
	UIDSubmitter    string  `json:"uid_submitter"`
	CommunityID     *string `json:"community_id,omitempty"`
	DescriptionText string  `json:"description_text"`
	StartTimeUTC    string  `json:"start_time_utc"`
	EndTimeUTC      string  `json:"end_time_utc"`
	LocationHash    *string `json:"location_hash,omitempty"`
	CreatedAtUTC    string  `json:"created_at_utc"`
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := s.decodeBody(r, &req); err != nil {
		s.errorJSON(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.UIDSubmitter == "" || req.DescriptionText == "" ||
		req.StartTimeUTC == "" || req.EndTimeUTC == "" {
		s.errorJSON(w, http.StatusBadRequest,
			"uid_submitter, description_text, start_time_utc, end_time_utc are required")
		return
	}

	start, err := time.Parse(time.RFC3339, req.StartTimeUTC)
	if err != nil {
		s.errorJSON(w, http.StatusBadRequest, "start_time_utc must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, req.EndTimeUTC)
	if err != nil {
		s.errorJSON(w, http.StatusBadRequest, "end_time_utc must be RFC3339")
		return
	}
	start, end = start.UTC(), end.UTC()

	taskID := uuid.New().String()

	// R-004: inverted timestamps — pre_execution
	res4, err := r004.Check(r.Context(), nil, r004.Input{
		TaskID: taskID, StartTimeUTC: start, EndTimeUTC: end,
	})
	if err != nil {
		s.Logger.Error("R-004 check", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "rule check error")
		return
	}
	if res4.Event.Verdict.Result == audit.VerdictBlock {
		_ = audit.Store(r.Context(), s.Pool, res4.Event)
		s.writeJSON(w, http.StatusConflict, BlockedResponse{
			Error: "blocked", RuleID: r004.RuleID, RuleVersion: "1.0",
			Reason:       derefStr(res4.Event.Evidence.MatchedPattern),
			AuditEventID: res4.Event.EventID,
		})
		return
	}

	// R-003: duration ceiling — pre_execution
	res3, err := r003.Check(r.Context(), nil, r003.Input{
		TaskID: taskID, StartTimeUTC: start, EndTimeUTC: end,
	})
	if err != nil {
		s.Logger.Error("R-003 check", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "rule check error")
		return
	}
	if res3.Event.Verdict.Result == audit.VerdictBlock {
		_ = audit.Store(r.Context(), s.Pool, res3.Event)
		s.writeJSON(w, http.StatusConflict, BlockedResponse{
			Error: "blocked", RuleID: r003.RuleID, RuleVersion: "1.0",
			Reason:       derefStr(res3.Event.Evidence.MatchedPattern),
			AuditEventID: res3.Event.EventID,
		})
		return
	}

	// Insert task
	var createdAt time.Time
	err = s.Pool.QueryRow(r.Context(), `
		INSERT INTO tasks
			(task_id, uid_submitter, community_id, description_text,
			 start_time_utc, end_time_utc, location_hash)
		VALUES ($1::UUID, $2, $3::UUID, $4, $5, $6, $7)
		RETURNING created_at_utc`,
		taskID, req.UIDSubmitter, req.CommunityID, req.DescriptionText,
		start, end, req.LocationHash,
	).Scan(&createdAt)
	if err != nil {
		s.Logger.Error("insert task", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	// Persist PASS audit events (idempotent)
	_ = audit.Store(r.Context(), s.Pool, res4.Event)
	_ = audit.Store(r.Context(), s.Pool, res3.Event)

	s.writeJSON(w, http.StatusCreated, taskResponse{
		TaskID:          taskID,
		UIDSubmitter:    req.UIDSubmitter,
		CommunityID:     req.CommunityID,
		DescriptionText: req.DescriptionText,
		StartTimeUTC:    start.Format(time.RFC3339),
		EndTimeUTC:      end.Format(time.RFC3339),
		LocationHash:    req.LocationHash,
		CreatedAtUTC:    createdAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		s.errorJSON(w, http.StatusBadRequest, "task_id required")
		return
	}

	var res taskResponse
	var start, end, createdAt time.Time
	err := s.Pool.QueryRow(r.Context(), `
		SELECT task_id::TEXT, uid_submitter, community_id::TEXT,
		       description_text, start_time_utc, end_time_utc,
		       location_hash, created_at_utc
		FROM   tasks
		WHERE  task_id = $1::UUID`,
		taskID,
	).Scan(
		&res.TaskID, &res.UIDSubmitter, &res.CommunityID,
		&res.DescriptionText, &start, &end,
		&res.LocationHash, &createdAt,
	)
	if err != nil {
		if isNotFound(err) {
			s.errorJSON(w, http.StatusNotFound, "task not found")
		} else {
			s.Logger.Error("get task", "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
		}
		return
	}

	res.StartTimeUTC = start.UTC().Format(time.RFC3339)
	res.EndTimeUTC = end.UTC().Format(time.RFC3339)
	res.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
	s.writeJSON(w, http.StatusOK, res)
}
