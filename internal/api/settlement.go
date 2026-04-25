package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/audit"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r001"
	"github.com/Yvaine-Xyan/linkrsp/internal/rules/r005"
)

// vBitMap implements V_bit from the LRS-1.0 algorithm spec §2.1.
var vBitMap = map[int]float64{0: 0.1, 1: 0.5, 2: 1.0}

// FormulaSnapshot captures all LRS-1.0 inputs and outputs for the audit trail.
type FormulaSnapshot struct {
	TPhyMinutes  float64 `json:"t_phy_minutes"`
	VBit         float64 `json:"v_bit"`
	DBase        float64 `json:"d_base"`  // frozen at 1.0 per C-3
	WRisk        float64 `json:"w_risk"`  // 0.0 in Phase B
	KGlobal      float64 `json:"k_global"` // 1.0 genesis period per C-4
	ClipResult   float64 `json:"clip_result"`
	CreditsDelta float64 `json:"credits_delta"`
}

// computeCredits evaluates the LRS-1.0 formula with Phase B defaults.
// D_base=1.0 (C-3 frozen), K_global=1.0 (C-4 genesis), W_risk=0.
func computeCredits(tPhyMinutes float64, verificationLevel int) FormulaSnapshot {
	vBit := vBitMap[verificationLevel]
	const dBase, wRisk, kGlobal = 1.0, 0.0, 1.0
	raw := (dBase + wRisk) / kGlobal
	clip := math.Max(0.8, math.Min(3.0, raw))
	return FormulaSnapshot{
		TPhyMinutes:  tPhyMinutes,
		VBit:         vBit,
		DBase:        dBase,
		WRisk:        wRisk,
		KGlobal:      kGlobal,
		ClipResult:   clip,
		CreditsDelta: tPhyMinutes * vBit * clip,
	}
}

type settlementResponse struct {
	TaskID        string          `json:"task_id"`
	UID           string          `json:"uid"`
	RulesChecked  []string        `json:"rules_checked"`
	Verdict       string          `json:"verdict"`
	Formula       FormulaSnapshot `json:"formula_snapshot"`
	LedgerEntryID *string         `json:"ledger_entry_id,omitempty"` // only on commit
}

func (s *Server) runSettlement(w http.ResponseWriter, r *http.Request, commit bool) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		s.errorJSON(w, http.StatusBadRequest, "task_id required")
		return
	}

	// Load task + max attestation level in one query.
	var uid string
	var start, end time.Time
	var locationHash *string
	var verificationLevel int
	err := s.Pool.QueryRow(r.Context(), `
		SELECT t.uid_submitter, t.start_time_utc, t.end_time_utc, t.location_hash,
		       COALESCE(MAX(a.verification_level), 0)
		FROM   tasks t
		LEFT   JOIN attestations a ON a.task_id = t.task_id
		WHERE  t.task_id = $1::UUID
		GROUP  BY t.uid_submitter, t.start_time_utc, t.end_time_utc, t.location_hash`,
		taskID,
	).Scan(&uid, &start, &end, &locationHash, &verificationLevel)
	if err != nil {
		if isNotFound(err) {
			s.errorJSON(w, http.StatusNotFound, "task not found")
		} else {
			s.Logger.Error("load task settlement", "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
		}
		return
	}
	start, end = start.UTC(), end.UTC()

	var auditEvents []audit.Event

	// R-001: physical paradox — post_execution.
	res1, err := r001.Check(r.Context(), s.Pool, r001.Input{
		UID:               uid,
		TaskID:            taskID,
		StartTimeUTC:      start,
		EndTimeUTC:        end,
		LocationHash:      derefStr(locationHash),
		VerificationLevel: verificationLevel,
	})
	if err != nil {
		s.Logger.Error("R-001 check", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "rule check error")
		return
	}
	auditEvents = append(auditEvents, res1.Event)
	if res1.Event.Verdict.Result == audit.VerdictBlock {
		if commit {
			_ = audit.Store(r.Context(), s.Pool, res1.Event)
		}
		s.writeJSON(w, http.StatusConflict, BlockedResponse{
			Error: "blocked", RuleID: r001.RuleID, RuleVersion: "1.0",
			Reason:       derefStr(res1.Event.Evidence.MatchedPattern),
			AuditEventID: res1.Event.EventID,
		})
		return
	}

	// R-005: daily hour ceiling — post_execution (WARN only, does not block).
	res5, err := r005.Check(r.Context(), s.Pool, r005.Input{
		UID:          uid,
		TaskID:       taskID,
		WindowEndUTC: end,
	})
	if err != nil {
		s.Logger.Error("R-005 check", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "rule check error")
		return
	}
	auditEvents = append(auditEvents, res5.Event)

	tPhy := end.Sub(start).Minutes()
	formula := computeCredits(tPhy, verificationLevel)

	resp := settlementResponse{
		TaskID:       taskID,
		UID:          uid,
		RulesChecked: []string{r001.RuleID, r005.RuleID},
		Verdict:      "PASS",
		Formula:      formula,
	}

	if !commit {
		s.writeJSON(w, http.StatusOK, resp)
		return
	}

	// Commit: idempotent ledger entry (append-only, C-6).
	idKey := fmt.Sprintf("%s|settlement", taskID)
	formulaJSON, _ := json.Marshal(formula)

	// Try insert first; if conflict (already settled) fetch existing entry_id.
	var entryID string
	insertErr := s.Pool.QueryRow(r.Context(), `
		INSERT INTO ledger_entries
			(task_id, uid, credits_delta, formula_snapshot, idempotency_key)
		VALUES ($1::UUID, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING entry_id::TEXT`,
		taskID, uid, formula.CreditsDelta, formulaJSON, idKey,
	).Scan(&entryID)
	if insertErr != nil {
		// No row returned = conflict; fetch the existing entry.
		if isNotFound(insertErr) {
			fetchErr := s.Pool.QueryRow(r.Context(),
				`SELECT entry_id::TEXT FROM ledger_entries WHERE idempotency_key = $1`,
				idKey,
			).Scan(&entryID)
			if fetchErr != nil {
				s.Logger.Error("fetch existing ledger entry", "error", fetchErr)
				s.errorJSON(w, http.StatusInternalServerError, "database error")
				return
			}
		} else {
			s.Logger.Error("insert ledger entry", "error", insertErr)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	for _, ev := range auditEvents {
		_ = audit.Store(r.Context(), s.Pool, ev)
	}

	resp.LedgerEntryID = &entryID
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) previewSettlement(w http.ResponseWriter, r *http.Request) {
	s.runSettlement(w, r, false)
}

func (s *Server) commitSettlement(w http.ResponseWriter, r *http.Request) {
	s.runSettlement(w, r, true)
}
