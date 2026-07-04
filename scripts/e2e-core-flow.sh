#!/usr/bin/env bash
# E2E core-flow functional test: drives a real daemon + Claude agent through
# the full core loop and asserts each step in the DB. See
# docs/superpowers/specs/2026-07-04-e2e-core-flow-test-design.md
set -euo pipefail

# ---------- Config ----------
API_BASE="${API_BASE:-http://localhost:8080}"
WORKSPACE_ID="8279ae9b-16f5-4904-92f0-b19fd8e18c5d"
CLAUDE_RUNTIME_ID="019f2b3c-3971-79e0-b588-be9e048591b4"
CEO_TEMPLATE_NAME='👔 CEO · 缤果软件'
WIKI_SPACE_SLUG="e2e-core-flow"
WIKI_PAGE_PATH="wiki/conventions.md"
DEV_CODE="${MULTICA_DEV_VERIFICATION_CODE:-123456}"
REPORT_FILE="/tmp/e2e-core-flow-report.md"
LOG_FILE="/tmp/e2e-core-flow.log"
DAEMON_PID=""
TOKEN=""

# ---------- Helpers ----------
log()  { printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*" | tee -a "$LOG_FILE"; }
die()  { log "ERROR: $*"; exit 1; }

psql_at() { psql "$DATABASE_URL" -At -F $'\t' --no-psqlrc -c "$1"; }

assert_count_ge() {
  # $1 = label, $2 = expected min, $3 = sql
  local label="$1" expected="$2" sql="$3"
  local actual
  actual="$(psql_at "$sql")"
  actual="${actual:-0}"
  if (( actual >= expected )); then
    log "  ✓ $label: $actual (>= $expected)"
  else
    die "  ✗ $label: expected >= $expected, got $actual"
  fi
}

assert_exists() {
  # $1 = label, $2 = sql (must return non-empty)
  local label="$1" sql="$2"
  local result
  result="$(psql_at "$sql")"
  if [[ -n "$result" ]]; then
    log "  ✓ $label: $result"
  else
    die "  ✗ $label: expected non-empty result"
  fi
}

api_get()  { curl -sS -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" "$API_BASE$1"; }
api_post() { curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" -H "Content-Type: application/json" -d "$2" "$API_BASE$1"; }
api_put()  { curl -sS -X PUT  -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" -H "Content-Type: application/json" -d "$2" "$API_BASE$1"; }

# ---------- Main ----------
main() {
  local stop_phase=""
  local cleanup=0
  keep_daemon=0  # global: teardown (trap) reads this via ${keep_daemon:-0}
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --phase) stop_phase="$2"; shift 2;;
      --cleanup) cleanup=1; shift;;
      --keep-daemon) keep_daemon=1; shift;;
      --help|-h)
        cat <<EOF
Usage: scripts/e2e-core-flow.sh [--phase <name>] [--cleanup] [--keep-daemon]
Phases: preflight auth daemon oss wiki ceo trigger poll evidence teardown
--phase NAME   stop after the named phase (for incremental testing)
--cleanup      delete test data (issue/agents/wiki space/oss objects) on exit
--keep-daemon  do not kill the daemon started by this script
EOF
        exit 0;;
      *) die "unknown arg: $1";;
    esac
  done
  : > "$LOG_FILE"
  log "=== E2E core-flow test started ==="
  trap teardown EXIT
  run_phases "$stop_phase"
  if [[ "$cleanup" == "1" ]]; then CLEANUP=1; fi
}

# ---------- Phases ----------
phase_preflight() {
  log "=== Phase 0: Preflight ==="
  command -v curl >/dev/null || die "curl not found"
  command -v jq   >/dev/null || die "jq not found"
  command -v psql >/dev/null || die "psql not found"
  command -v claude >/dev/null || die "claude CLI not found"
  curl -sS -o /dev/null -w "" "$API_BASE/healthz" || die "backend not reachable at $API_BASE (start: make dev)"
  log "  backend reachable"
  OWNER_EMAIL="$(psql_at "SELECT u.email FROM member m JOIN \"user\" u ON u.id=m.user_id WHERE m.workspace_id='$WORKSPACE_ID' AND m.role='owner' LIMIT 1")"
  [[ -n "$OWNER_EMAIL" ]] || die "no owner found for workspace $WORKSPACE_ID"
  log "  owner: $OWNER_EMAIL"
}

phase_auth() {
  log "=== Phase 1: Auth ==="
  # Clear stale codes to avoid per-email send-code rate limit.
  psql_at "DELETE FROM verification_code WHERE email='$OWNER_EMAIL'" >/dev/null || true
  api_post /auth/send-code "{\"email\":\"$OWNER_EMAIL\"}" >/dev/null
  local resp
  resp="$(api_post /auth/verify-code "{\"email\":\"$OWNER_EMAIL\",\"code\":\"$DEV_CODE\"}")"
  TOKEN="$(printf '%s' "$resp" | jq -r '.token // empty')"
  [[ -n "$TOKEN" ]] || die "no token in verify-code response: $resp"
  log "  JWT obtained"
}

# ---------- Dispatcher ----------
run_phases() {
  local stop_after="$1"
  local phases=(preflight auth daemon oss wiki ceo trigger poll evidence teardown)
  local p
  for p in "${phases[@]}"; do
    "phase_$p"
    [[ "$p" == "$stop_after" ]] && { log "=== stopped after $p ==="; return 0; }
  done
}

# ---------- Teardown (stub; filled in Task 8) ----------
CLEANUP=0
teardown() {
  local rc=$?
  if [[ -n "$DAEMON_PID" && "$keep_daemon" == 0 ]]; then
    kill "$DAEMON_PID" 2>/dev/null || true
  fi
  # evidence + cleanup added in Task 8
  log "=== exit $rc ==="
}

main "$@"
