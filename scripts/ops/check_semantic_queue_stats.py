#!/usr/bin/env python3
"""
Check semantic_audit_jobs queue stats via LinkRSP internal API.
Exit 0: within thresholds. Exit 1: backlog or age exceeded. Exit 0: skip if BASE_URL unset.

Env:
  BASE_URL              - e.g. https://api.example.com (no trailing slash)
  SECRET                - X-API-Shared-Secret (optional if server allows empty)
  MAX_PENDING           - max allowed pending count (default 200)
  MAX_HUMAN_REVIEW      - max allowed human_review count (default 200)
  OLDEST_PENDING_MAX_AGE_HOURS - fail if oldest pending older than this (default 72)
  OLDEST_HUMAN_MAX_AGE_HOURS   - fail if oldest human_review older than this (default 72)
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone


def main() -> int:
    base = os.environ.get("BASE_URL", "").strip().rstrip("/")
    if not base:
        print("BASE_URL not set — skipping semantic queue watchdog")
        return 0

    secret = os.environ.get("SECRET", "")

    def getenv_int(name: str, default: int) -> int:
        v = os.environ.get(name, "").strip()
        if not v:
            return default
        return int(v)

    def getenv_float(name: str, default: float) -> float:
        v = os.environ.get(name, "").strip()
        if not v:
            return default
        return float(v)

    max_pending = getenv_int("MAX_PENDING", 200)
    max_human = getenv_int("MAX_HUMAN_REVIEW", 200)
    max_age_p = getenv_float("OLDEST_PENDING_MAX_AGE_HOURS", 72.0)
    max_age_h = getenv_float("OLDEST_HUMAN_MAX_AGE_HOURS", 72.0)

    url = f"{base}/api/v1/internal/semantic-audit-jobs/stats"
    req = urllib.request.Request(url, method="GET")
    if secret:
        req.add_header("X-API-Shared-Secret", secret)

    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            raw = resp.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        print(f"HTTP {e.code}: {body}", file=sys.stderr)
        return 1
    except urllib.error.URLError as e:
        print(f"request failed: {e}", file=sys.stderr)
        return 1

    data = json.loads(raw)
    counts = data.get("counts_by_status") or {}
    pending = int(counts.get("pending", 0))
    human = int(counts.get("human_review", 0))

    print(json.dumps(data, indent=2))

    errors: list[str] = []
    if pending > max_pending:
        errors.append(f"pending count {pending} > MAX_PENDING {max_pending}")
    if human > max_human:
        errors.append(f"human_review count {human} > MAX_HUMAN_REVIEW {max_human}")

    now = datetime.now(timezone.utc)

    def check_age(field: str, iso: str | None, max_hours: float, label: str) -> None:
        if not iso or max_hours <= 0:
            return
        try:
            ts = datetime.fromisoformat(iso.replace("Z", "+00:00"))
            if ts.tzinfo is None:
                ts = ts.replace(tzinfo=timezone.utc)
            age_h = (now - ts).total_seconds() / 3600.0
            if age_h > max_hours:
                errors.append(
                    f"{label}: oldest {field} is {age_h:.1f}h old (> {max_hours}h), ts={iso}"
                )
        except ValueError:
            errors.append(f"invalid timestamp for {field}: {iso!r}")

    check_age("oldest_pending_utc", data.get("oldest_pending_utc"), max_age_p, "pending")
    check_age("oldest_human_review_utc", data.get("oldest_human_review_utc"), max_age_h, "human_review")

    if errors:
        for e in errors:
            print(f"WATCHDOG_FAIL: {e}", file=sys.stderr)
        return 1

    print("WATCHDOG_OK: within thresholds")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
