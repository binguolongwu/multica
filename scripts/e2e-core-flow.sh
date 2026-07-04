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

phase_daemon() {
  log "=== Phase 2: start daemon + wait for Claude runtime ==="
  # Only start if not already running.
  if pgrep -f "cmd/daemon" >/dev/null 2>&1; then
    log "  daemon already running, reusing"
  else
    (cd server && make daemon) >"$LOG_FILE.daemon" 2>&1 &
    DAEMON_PID=$!
    log "  daemon started (pid $DAEMON_PID)"
  fi
  # Poll until the Claude runtime's last_seen_at is recent and status != offline.
  local deadline=$(( $(date +%s) + 60 ))
  while [[ $(date +%s) -lt $deadline ]]; do
    local row
    row="$(psql_at "SELECT status, EXTRACT(EPOCH FROM (now()-last_seen_at)) FROM agent_runtime WHERE id='$CLAUDE_RUNTIME_ID'")"
    local status age
    status="$(printf '%s' "$row" | cut -f1)"
    age="$(printf '%s' "$row" | cut -f2)"
    if [[ "$status" != "offline" && -n "$age" ]] && awk "BEGIN{exit !($age < 30)}"; then
      log "  runtime online (status=$status, age=${age}s)"
      return 0
    fi
    sleep 3
  done
  die "Claude runtime $CLAUDE_RUNTIME_ID did not come online within 60s — check $LOG_FILE.daemon and MULTICA_CODEX_PATH / claude CLI auth"
}

phase_oss() {
  log "=== Phase 3: verify 七牛华北 OSS config ==="
  local row
  row="$(psql_at "SELECT provider, region, is_default, custom_domain, folder_prefix FROM oss_provider_config WHERE name='七牛华北' AND workspace_id='$WORKSPACE_ID'")"
  [[ -n "$row" ]] || die "七牛华北 OSS config not found in workspace $WORKSPACE_ID"
  local provider region is_default
  provider="$(printf '%s' "$row" | cut -f1)"
  region="$(printf '%s' "$row" | cut -f2)"
  is_default="$(printf '%s' "$row" | cut -f3)"
  [[ "$provider" == "qiniu" ]]      || die "expected provider=qiniu, got $provider"
  [[ "$region" == "z1" ]]           || die "expected region=z1, got $region"
  [[ "$is_default" == "t" ]]        || die "七牛华北 is not default"
  log "  OSS config OK (qiniu, z1, default, folder_prefix=$(printf '%s' "$row" | cut -f5))"
}

phase_wiki() {
  log "=== Phase 4: seed wiki ==="
  local space
  space="$(api_get "/api/wiki/spaces/$WIKI_SPACE_SLUG")"
  if printf '%s' "$space" | jq -e '.slug // empty' >/dev/null 2>&1; then
    log "  wiki space exists, reusing"
  else
    api_post /api/wiki/spaces "{\"slug\":\"$WIKI_SPACE_SLUG\",\"display_name\":\"E2E Core Flow\",\"access_scope\":\"shared\",\"template\":\"general\"}" >/dev/null
    log "  wiki space created"
  fi
  local body
  body=$(cat <<JSON
{"content":"# 项目约定\n## 模块命名\n模块名使用 kebab-case。\n## 文档路径\n产出存 projects/{project_id}/tasks/{task_id}/docs/api-design.md。"}
JSON
)
  api_put "/api/wiki/spaces/$WIKI_SPACE_SLUG/pages/$WIKI_PAGE_PATH" "$body" >/dev/null
  assert_exists "wiki page seeded" \
    "SELECT content_hash FROM wiki_page WHERE space_id=(SELECT id FROM wiki_space WHERE slug='$WIKI_SPACE_SLUG') AND path='$WIKI_PAGE_PATH'"
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
