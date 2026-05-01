package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type jobItem struct {
	JobID        string    `json:"job_id"`
	RuleID       string    `json:"rule_id"`
	SubjectType  string    `json:"subject_type"`
	SubjectID    string    `json:"subject_id"`
	TextRef      *string   `json:"text_ref"`
	Status       string    `json:"status"`
	Verdict      *string   `json:"verdict"`
	Confidence   *float64  `json:"confidence"`
	TriggerWords []string  `json:"trigger_words"`
	RoutedTo     *string   `json:"routed_to"`
	CreatedAtUTC time.Time `json:"created_at_utc"`
	UpdatedAtUTC time.Time `json:"updated_at_utc"`
}

type listResp struct {
	Jobs  []jobItem `json:"jobs"`
	Total int       `json:"total"`
}

type patchReq struct {
	Status     *string  `json:"status,omitempty"`
	Verdict    *string  `json:"verdict,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "list":
		cmdList(os.Args[2:])
	case "patch":
		cmdPatch(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `semreview: minimal Phase D semantic-audit reviewer CLI

Usage:
  semreview list  --base-url http://localhost:9090 --secret <shared> --status pending --limit 50
  semreview patch --base-url http://localhost:9090 --secret <shared> --job-id <uuid> --status done --verdict PASS --confidence 0.7

Notes:
  - This CLI does not define community SOP; it only operates the queue/write-back path.
  - PATCH write-back appends an audit_event (subject_type=semantic_audit_job) for replay.`)
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	baseURL := fs.String("base-url", "http://localhost:9090", "API base URL")
	secret := fs.String("secret", "", "X-API-Shared-Secret (optional if server allows empty)")
	status := fs.String("status", "pending", "Job status filter (pending|processing|human_review|done|skipped)")
	ruleID := fs.String("rule-id", "", "Optional rule_id filter")
	limit := fs.Int("limit", 50, "Max jobs (1-200)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "invalid args for list")
		os.Exit(2)
	}

	u := strings.TrimRight(*baseURL, "/") + "/api/v1/internal/semantic-audit-jobs?status=" + urlQ(*status) + "&limit=" + fmt.Sprint(*limit)
	if *ruleID != "" {
		u += "&rule_id=" + urlQ(*ruleID)
	}

	req, _ := http.NewRequest(http.MethodGet, u, nil)
	if *secret != "" {
		req.Header.Set("X-API-Shared-Secret", *secret)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "HTTP %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var out listResp
	if err := json.Unmarshal(body, &out); err != nil {
		fatal(err)
	}

	fmt.Printf("total=%d\n", out.Total)
	for _, j := range out.Jobs {
		fmt.Printf("- job_id=%s rule_id=%s subject=%s/%s status=%s", j.JobID, j.RuleID, j.SubjectType, j.SubjectID, j.Status)
		if j.Verdict != nil {
			fmt.Printf(" verdict=%s", *j.Verdict)
		}
		if j.Confidence != nil {
			fmt.Printf(" confidence=%.2f", *j.Confidence)
		}
		if len(j.TriggerWords) > 0 {
			fmt.Printf(" trigger_words=%v", j.TriggerWords)
		}
		if j.TextRef != nil {
			short := *j.TextRef
			if len(short) > 140 {
				short = short[:140] + "…"
			}
			fmt.Printf("\n  text_ref=%q", short)
		}
		fmt.Print("\n")
	}
}

func cmdPatch(args []string) {
	fs := flag.NewFlagSet("patch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	baseURL := fs.String("base-url", "http://localhost:9090", "API base URL")
	secret := fs.String("secret", "", "X-API-Shared-Secret (optional if server allows empty)")
	jobID := fs.String("job-id", "", "Semantic audit job_id (UUID)")
	status := fs.String("status", "", "New status (optional)")
	verdict := fs.String("verdict", "", "New verdict: PASS|BLOCK|SKIP (optional)")
	conf := fs.Float64("confidence", -1, "New confidence 0..1 (optional; omit by leaving at -1)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "invalid args for patch")
		os.Exit(2)
	}

	if *jobID == "" {
		fmt.Fprintln(os.Stderr, "--job-id is required")
		os.Exit(2)
	}

	var reqBody patchReq
	if *status != "" {
		reqBody.Status = status
	}
	if *verdict != "" {
		v := strings.ToUpper(*verdict)
		reqBody.Verdict = &v
	}
	if *conf >= 0 {
		reqBody.Confidence = conf
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		fatal(err)
	}

	u := strings.TrimRight(*baseURL, "/") + "/api/v1/internal/semantic-audit-jobs/" + *jobID
	req, _ := http.NewRequest(http.MethodPatch, u, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if *secret != "" {
		req.Header.Set("X-API-Shared-Secret", *secret)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "HTTP %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	fmt.Println(string(body))
}

func urlQ(s string) string {
	// minimal query escaping (enough for rule_id/status in our current use)
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), "+", "%2B")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
