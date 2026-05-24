package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/YvaineHe/linkrsp/internal/audit"
	"github.com/YvaineHe/linkrsp/internal/rules/r001"
	"github.com/YvaineHe/linkrsp/internal/rules/r005"
	"github.com/YvaineHe/linkrsp/internal/rules/r006"
	"github.com/YvaineHe/linkrsp/internal/rules/r010"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// vBitMap implements V_bit from the LRS-1.0 algorithm spec §2.1.
var vBitMap = map[int]float64{0: 0.1, 1: 0.5, 2: 1.0}

// FormulaSnapshot captures all LRS-1.0 inputs and outputs for the audit trail.
type FormulaSnapshot struct {
	TPhyMinutes  float64 `json:"t_phy_minutes"`
	VBit         float64 `json:"v_bit"`
	DBase        float64 `json:"d_base"`   // frozen at 1.0 per C-3
	WRisk        float64 `json:"w_risk"`   // 0.0 in Phase B
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
	var communityID *string
	var locationHash *string
	var verificationLevel int
	err := s.Pool.QueryRow(r.Context(), `
		SELECT t.uid_submitter, t.community_id::TEXT, t.start_time_utc, t.end_time_utc, t.location_hash,
		       COALESCE(MAX(a.verification_level), 0)
		FROM   tasks t
		LEFT   JOIN attestations a ON a.task_id = t.task_id
		WHERE  t.task_id = $1::UUID
		GROUP  BY t.uid_submitter, t.community_id, t.start_time_utc, t.end_time_utc, t.location_hash`,
		taskID,
	).Scan(&uid, &communityID, &start, &end, &locationHash, &verificationLevel)
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
			if err := audit.Store(r.Context(), s.Pool, res1.Event); err != nil {
				s.Logger.Error("store R-001 block audit event", "error", err)
			}
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

	// R-006: credit acceleration anomaly — post_execution (FLAG only, does not block).
	res6, err := r006.Check(r.Context(), s.Pool, r006.Input{
		UID:                   uid,
		CommunityID:           derefStr(communityID),
		AccelerationThreshold: s.R006AccelerationThreshold,
	})
	if err != nil {
		s.Logger.Error("R-006 check", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "rule check error")
		return
	}
	// Store a task-scoped event so /api/v1/audit-events?subject_type=task&subject_id=...
	// includes R-006 in the same subject stream as other task-level rules.
	ev6 := res6.AuditEvent
	ev6.EventID = uuid.New().String()
	ev6.Trace.TraceID = uuid.New().String()
	ev6.Subject = audit.Subject{Type: "task", ID: taskID, SecondaryID: &uid}
	auditEvents = append(auditEvents, ev6)

	tPhy := end.Sub(start).Minutes()
	formula := computeCredits(tPhy, verificationLevel)

	// R-010: post-genesis p99 credit outlier — post_execution (FLAG only, does not block).
	res10, err := r010.Check(r.Context(), s.Pool, r010.Input{
		UID:            uid,
		GenesisEndTime: s.R010GenesisEndTime,
		CurrentTime:    end,
	})
	if err != nil {
		s.Logger.Error("R-010 check", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "rule check error")
		return
	}
	// Store a task-scoped event so /api/v1/audit-events?... includes R-010.
	ev10 := res10.AuditEvent
	ev10.EventID = uuid.New().String()
	ev10.Trace.TraceID = uuid.New().String()
	ev10.Subject = audit.Subject{Type: "task", ID: taskID, SecondaryID: &uid}
	auditEvents = append(auditEvents, ev10)

	resp := settlementResponse{
		TaskID:       taskID,
		UID:          uid,
		RulesChecked: []string{r001.RuleID, r005.RuleID, r006.RuleID, r010.RuleID},
		Verdict:      "PASS",
		Formula:      formula,
	}

	if !commit {
		s.writeJSON(w, http.StatusOK, resp)
		return
	}

	// Commit: ledger + audit events in one transaction.
	idKey := fmt.Sprintf("%s|settlement", taskID)
	formulaJSON, err := json.Marshal(formula)
	if err != nil {
		s.Logger.Error("marshal settlement formula", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.Logger.Error("begin settlement transaction", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var entryID string
	if _, err := tx.Exec(r.Context(), "SAVEPOINT sp_ledger_insert"); err != nil {
		s.Logger.Error("savepoint ledger insert", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}
	insertErr := tx.QueryRow(r.Context(), `
		INSERT INTO ledger_entries
			(task_id, uid, credits_delta, formula_snapshot, idempotency_key)
		VALUES ($1::UUID, $2, $3, $4, $5)
		RETURNING entry_id::TEXT`,
		taskID, uid, formula.CreditsDelta, formulaJSON, idKey,
	).Scan(&entryID)
	if insertErr != nil {
		var pgErr *pgconn.PgError
		if errors.As(insertErr, &pgErr) && pgErr.Code == "23505" {
			// Idempotency: unique constraint hit, fetch existing entry.
			if _, err := tx.Exec(r.Context(), "ROLLBACK TO SAVEPOINT sp_ledger_insert"); err != nil {
				s.Logger.Error("rollback to savepoint ledger insert", "error", err)
				s.errorJSON(w, http.StatusInternalServerError, "database error")
				return
			}
			fetchErr := tx.QueryRow(r.Context(),
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
	if _, err := tx.Exec(r.Context(), "RELEASE SAVEPOINT sp_ledger_insert"); err != nil {
		s.Logger.Error("release savepoint ledger insert", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
	}

	for _, ev := range auditEvents {
		if err := audit.StoreWithExecer(r.Context(), tx, ev); err != nil {
			s.Logger.Error("store settlement audit event", "rule_id", ev.Rule.RuleID, "error", err)
			s.errorJSON(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil && err != pgx.ErrTxClosed {
		s.Logger.Error("commit settlement transaction", "error", err)
		s.errorJSON(w, http.StatusInternalServerError, "database error")
		return
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
