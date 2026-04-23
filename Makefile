# LinkRSP — Development Task Runner
# Usage: make <target>
#
# Phase A targets: available now (no Go required)
# Phase B+ targets: require go.mod to exist

.PHONY: help rule-lint rule-lint-all spec-check \
        fmt vet build test lint \
        audit-gen migrate-up migrate-down \
        ci-local clean

# ─── Default ──────────────────────────────────────────────────────
help:
	@echo ""
	@echo "  LinkRSP Make Targets"
	@echo "  ─────────────────────────────────────────────"
	@echo "  Phase A (available now):"
	@echo "    rule-lint         Validate all rule YAMLs"
	@echo "    rule-lint-one F=  Validate a single rule YAML"
	@echo "    spec-check        Check spec doc cross-references (Claude)"
	@echo ""
	@echo "  Phase B+ (requires Go):"
	@echo "    fmt               Run go fmt on all packages"
	@echo "    vet               Run go vet on all packages"
	@echo "    build             Build all binaries"
	@echo "    test              Run all tests with race detector"
	@echo "    test-cover        Run tests with HTML coverage report"
	@echo "    lint              Run golangci-lint"
	@echo "    audit-gen         Generate audit event fixtures"
	@echo ""
	@echo "  Database (Phase B+):"
	@echo "    migrate-up        Apply pending migrations"
	@echo "    migrate-down      Roll back last migration"
	@echo ""
	@echo "  CI:"
	@echo "    ci-local          Run full CI checks locally"
	@echo "    clean             Remove build artifacts"
	@echo ""

# ─── Phase A: Rule & Spec Automation ──────────────────────────────

rule-lint:
	@echo "→ Validating all rule YAMLs..."
	@py .claude/scripts/validate_rule.py --all

rule-lint-one:
	@[ -n "$(F)" ] || (echo "Usage: make rule-lint-one F=docs/governance/rule-engine/r-class/R-001.yaml" && exit 1)
	@py .claude/scripts/validate_rule.py "$(F)"

rule-count:
	@echo "Rule YAML files:"
	@find docs/governance/rule-engine -name "*.yaml" | sort | while read f; do \
		echo "  $$f"; \
	done
	@echo ""
	@find docs/governance/rule-engine -name "*.yaml" | wc -l | xargs echo "Total:"

yaml-lint:
	@which yamllint >/dev/null 2>&1 || (echo "Install: pip install yamllint" && exit 1)
	@yamllint -d "{extends: relaxed, rules: {line-length: {max: 200}}}" docs/governance/rule-engine/

# ─── Phase B+: Go ──────────────────────────────────────────────────

GO_MODULE_EXISTS := $(shell test -f go.mod && echo yes || echo no)

fmt:
ifeq ($(GO_MODULE_EXISTS),yes)
	@echo "→ go fmt..."
	@go fmt ./...
else
	@echo "[skip] go.mod not found — Phase B not started"
endif

vet:
ifeq ($(GO_MODULE_EXISTS),yes)
	@echo "→ go vet..."
	@go vet ./...
else
	@echo "[skip] go.mod not found"
endif

build:
ifeq ($(GO_MODULE_EXISTS),yes)
	@echo "→ go build..."
	@go build ./...
else
	@echo "[skip] go.mod not found"
endif

test:
ifeq ($(GO_MODULE_EXISTS),yes)
	@echo "→ go test (race detector)..."
	@go test -race ./...
else
	@echo "[skip] go.mod not found"
endif

test-cover:
ifeq ($(GO_MODULE_EXISTS),yes)
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
else
	@echo "[skip] go.mod not found"
endif

lint:
ifeq ($(GO_MODULE_EXISTS),yes)
	@which golangci-lint >/dev/null 2>&1 || (echo "Install: https://golangci-lint.run/usage/install/" && exit 1)
	@golangci-lint run --timeout 5m
else
	@echo "[skip] go.mod not found"
endif

# Run govulncheck for known vulnerability scan
vuln:
ifeq ($(GO_MODULE_EXISTS),yes)
	@which govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	@govulncheck ./...
else
	@echo "[skip] go.mod not found"
endif

# ─── Database Migrations ──────────────────────────────────────────
# Convention: migrations live in db/migrations/
# Tool: golang-migrate (install separately)

migrate-up:
ifeq ($(GO_MODULE_EXISTS),yes)
	@[ -n "$(DATABASE_URL)" ] || (echo "Set DATABASE_URL first" && exit 1)
	@migrate -path db/migrations -database "$(DATABASE_URL)" up
else
	@echo "[skip] Phase B not started — no migrations yet"
endif

migrate-down:
ifeq ($(GO_MODULE_EXISTS),yes)
	@[ -n "$(DATABASE_URL)" ] || (echo "Set DATABASE_URL first" && exit 1)
	@migrate -path db/migrations -database "$(DATABASE_URL)" down 1
else
	@echo "[skip] Phase B not started"
endif

# ─── Audit Event Fixtures ─────────────────────────────────────────
# Generate sample audit event JSONs from rule YAMLs (useful for integration tests)
audit-gen:
	@echo "→ Generating audit event fixtures..."
	@py .claude/scripts/validate_rule.py --all
	@echo "[audit-gen] Fixture generation not yet implemented (Phase B task)"

# ─── CI Local ─────────────────────────────────────────────────────
ci-local: rule-lint yaml-lint fmt vet build test lint
	@echo ""
	@echo "✓ CI local checks complete"

# ─── Clean ────────────────────────────────────────────────────────
clean:
	@rm -f coverage.out coverage.html
ifeq ($(GO_MODULE_EXISTS),yes)
	@go clean ./...
endif
	@echo "✓ Clean"
