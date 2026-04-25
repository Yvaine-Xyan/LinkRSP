package api

import (
	"net/http"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r002"
	"github.com/google/uuid"
)

type createAttestationRequest struct {
	VerificationLevel int      `json:"verification_level"`
	TimestampUTC      string   `json:"timestamp_utc"`
	LocationLat       *float64 `json:"location_lat"`
	LocationLng       *float64 `json:"location_lng"`
	LocationHash      *string  `json:"location_hash"`
	EvidenceRef       *string  `json:"evidence_ref"`
}

type attestationResponse struct {
	AttestationID     string   `json:"attestation_id"`
	TaskID            string   `json:"task_id"`
	VerificationLevel int      `json:"verification_level"`
	TimestampUTC      string   `json:"timestamp_utc"`
	LocationLat       *float64 `json:"location_lat,omitempty"`
	LocationLng       *float64 `json:"location_lng,omitempty"`
	LocationHash      *string  `json:"location_hash,omitempty"`
	EvidenceRef       *string  `json:"evidence_ref,omitempty"`
	CreatedAtUTC      string   `json:"created_at_utc"`
}

func (s *Server) createAttestation(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		s.errorJSON(w, http.StatusBadRequest, "task_id required")
		return
	}

	var req createAttestationRequest
	if err := s.decodeBody(r, &req); err != nil {
		s.errorJSON(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.TimestampUTC == "" {
		s.errorJSON(w, http.StatusBadRequest, "timestamp_utc is required")
		return
	}
	if req.VerificationLevel < 0 || req.VerificationLevel > 2 {
		s.errorJSON(w, http.StatusBadRequest, "verification_level must be 0, 1, or 2")
		return
	}

	ts, err := time.Parse(time.RFC3339, req.TimestampUTC)
	if err != nil {
		s.errorJSON(w, http.StatusBadRequest, "timestamp_utc must be RFC3339")
		return
	}
	ts = ts.UTC()

	// Verify the task exists and get uid_submitter.
	var uid string
	err = s.Pool.QueryRow(r.Context(),
		`SELECT uid_submitter FROM tasks WHERE task_id = $1::UUID`, taskID,
	).Scan(&uid)
	if err != nil {
		if isNotFound(err) {
			s.errorJSON(w, http.StatusNotFound, "task not found")
		} else {
			s.Logger.Error("verify task", "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
		}
		return
	}

	// R-002: V=2 handshake speed — post_execution.
	// Runs before insert so we can block before touching the DB.
	lat := 0.0
	lng := 0.0
	if req.LocationLat != nil {
		lat = *req.LocationLat
	}
	if req.LocationLng != nil {
		lng = *req.LocationLng
	}
	res2, err := r002.Check(r.Context(), s.Pool, r002.Input{
		UID:                uid,
		TaskID:             taskID,
		VerificationLevel:  req.VerificationLevel,
		HandshakeTimestamp: ts,
		HandshakeLat:       lat,
		HandshakeLng:       lng,
	})
	if err != nil {
		s.Logger.Error("R-002 check", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "rule check error")
		return
	}
	if res2.Event.Verdict.Result == audit.VerdictBlock {
		_ = audit.Store(r.Context(), s.Pool, res2.Event)
		s.writeJSON(w, http.StatusConflict, BlockedResponse{
			Error: "blocked", RuleID: r002.RuleID, RuleVersion: "1.0",
			Reason:       derefStr(res2.Event.Evidence.MatchedPattern),
			AuditEventID: res2.Event.EventID,
		})
		return
	}

	// Insert attestation.
	attestationID := uuid.New().String()
	var createdAt time.Time
	err = s.Pool.QueryRow(r.Context(), `
		INSERT INTO attestations
			(attestation_id, task_id, verification_level, timestamp_utc,
			 location_lat, location_lng, location_hash, evidence_ref)
		VALUES ($1::UUID, $2::UUID, $3, $4, $5, $6, $7, $8)
		RETURNING created_at_utc`,
		attestationID, taskID, req.VerificationLevel, ts,
		req.LocationLat, req.LocationLng, req.LocationHash, req.EvidenceRef,
	).Scan(&createdAt)
	if err != nil {
		s.Logger.Error("insert attestation", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	_ = audit.Store(r.Context(), s.Pool, res2.Event)

	s.writeJSON(w, http.StatusCreated, attestationResponse{
		AttestationID:     attestationID,
		TaskID:            taskID,
		VerificationLevel: req.VerificationLevel,
		TimestampUTC:      ts.Format(time.RFC3339),
		LocationLat:       req.LocationLat,
		LocationLng:       req.LocationLng,
		LocationHash:      req.LocationHash,
		EvidenceRef:       req.EvidenceRef,
		CreatedAtUTC:      createdAt.UTC().Format(time.RFC3339),
	})
}
