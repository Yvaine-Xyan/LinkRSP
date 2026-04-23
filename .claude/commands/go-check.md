Review Go code files changed in this session for LinkRSP-specific quality requirements. Use this during Phase B and beyond whenever Go files are written or modified.

**Steps:**

1. **Identify changed Go files**: from session edits or from `git diff --name-only HEAD | grep '\.go$'`

2. **For each Go file, check**:

   **Error handling**
   - No `_ = err` without an explanatory comment
   - No `err != nil` checks that silently swallow errors (log or return)
   - HTTP handlers must return appropriate status codes on error

   **Context propagation**
   - Functions that make DB calls accept `context.Context` as first parameter
   - Context is passed through, not created fresh inside service functions
   - `context.Background()` only used at program entrypoints

   **Ledger / event sourcing invariants**
   - Any table whose name contains `event`, `ledger`, `audit`, `credit`: only `INSERT` allowed
   - No raw `UPDATE` or `DELETE` SQL on these tables
   - Settlement must be preview-then-commit (two separate operations)

   **Audit event generation**
   - Every rule execution path emits an audit event via the audit service
   - Audit events include the idempotency key fields: rule_id, subject_type, subject_id, trigger, timestamp_minute
   - S-class rules: confidence field non-nil in audit event

   **Security**
   - No hardcoded connection strings, API keys, or secrets
   - SQL queries use parameterized queries (no string interpolation into SQL)
   - User-controlled input not used directly in file paths

   **Idempotency**
   - Rule check functions: same inputs → same verdict (no side effects on read path)
   - Event writes use INSERT ON CONFLICT DO NOTHING with the idempotency key

   **LinkRSP credit formula**
   - Credits_delta computed only as: `T_phy * V_bit * Clip((D_base + sum_W_risk) / K_global, 0.8, 3.0)`
   - `D_base` constant = 1.0 in LRS-1.0 (not a variable parameter)
   - `Clip` lower bound = 0.8, upper bound = 3.0

3. **Run tooling if available**:
   ```bash
   go fmt ./...
   go vet ./...
   go build ./...
   ```
   Report any output from these commands.

4. **Output**:
   - `[ISSUE] file:line — description` for each problem
   - `[OK] check_name` for clean areas
   - `✓ Go checks passed` or `✗ N issue(s) found`
