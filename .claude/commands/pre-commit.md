Perform a structured pre-commit self-review of all changes made in this session. Produce a commit-ready summary and flag any issues that must be fixed first.

**Steps:**

1. **Inventory changes**: Run `git status` and `git diff --stat HEAD` to see exactly what changed.

2. **Read each changed file** (or diff) and check:

   **For any file — Critical Constraint scan (CLAUDE.md C-1 to C-8)**:
   - C-1: LRS credits not described as currency/money/token/payment
   - C-2: No protocol fiat custody implied
   - C-3: D_base not treated as configurable
   - C-4: K_global genesis lock not changed
   - C-5: All timestamps UTC, windows relative to task start_time
   - C-6: No UPDATE/DELETE on ledger/event tables (Go/SQL files)
   - C-7: No path from LLM output directly to credit increment
   - C-8: Rule YAML version not changed from "1.0"

   **For rule YAML files**: Run the full rule-lint checks (same as /rule-lint)

   **For spec documents**: Run the spec cross-reference checks (same as /spec-check)

   **For Go files** (Phase B+):
   - Error handling not silenced
   - Context propagation present
   - No hardcoded credentials
   - go fmt compliance (formatting)

   **For HTML/web files**:
   - Every visible text block has both `.zh-text` and `.en-text` variants
   - `www.linkrsp.com` canonical URL present in head
   - No broken links to doc files

3. **Draft commit message** following the project format:
   ```
   type(scope): short description in English

   - bullet describing what changed
   - bullet describing why if non-obvious
   ```
   Choose `type` from: feat | fix | docs | refactor | test | chore
   Choose `scope` from: rule-engine | spec | ops | engineering | go | api | db | web | automation

4. **Output**:
   - List of `[ISSUE] description` for anything that must be fixed
   - List of `[OK] description` for clean checks
   - Proposed commit message in a code block
   - Final verdict: `Ready to commit ✓` or `Fix N issue(s) before committing ✗`
