#!/usr/bin/env bash
# E2E core-flow functional test: drives a real daemon + Claude agent through
# the full core loop and asserts each step in the DB. See
# docs/superpowers/specs/2026-07-04-e2e-core-flow-test-design.md
#
# Prerequisites:
#   1. Backend running on :8080  (make server  or  make dev)
#   2. Claude CLI authed   (claude --version works)
#   3. DATABASE_URL env var set (pointing at multica_dev)
#   4. MULTICA_DEV_VERIFICATION_CODE=123456
#   5. Workspace 8279ae9b-... has: Claude runtime, CEO template, 七牛华北 OSS config
#
# Usage:
#   bash scripts/e2e-core-flow.sh                 # full run, keep test data
#   bash scripts/e2e-core-flow.sh --phase wiki    # stop after a phase
#   bash scripts/e2e-core-flow.sh --cleanup       # remove test data on exit
#   bash scripts/e2e-core-flow.sh --keep-daemon   # don't kill the daemon
#
# Outputs:
#   /tmp/e2e-core-flow.log        — full output (curl + psql)
#   /tmp/e2e-core-flow.log.daemon — daemon stdout/stderr
#   /tmp/e2e-core-flow-report.md  — structured markdown report
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

psql_at() {
  # Run a SQL query; retry once on failure (the remote dev DB occasionally drops idle connections).
  local out rc
  out="$(psql "$DATABASE_URL" -At -F $'\t' --no-psqlrc -c "$1" 2>/dev/null)"; rc=$?
  if [[ $rc -ne 0 ]]; then
    sleep 2
    out="$(psql "$DATABASE_URL" -At -F $'\t' --no-psqlrc -c "$1" 2>/dev/null)" || true
  fi
  printf '%s' "$out"
}

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
    make daemon >"$LOG_FILE.daemon" 2>&1 &
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

phase_ceo() {
  log "=== Phase 5: create CEO agent from template ==="
  # Resolve template UUID by name (do NOT hardcode the UUID — it may differ across DBs).
  local template_id
  template_id="$(psql_at "SELECT id::text FROM agent_template WHERE name='$CEO_TEMPLATE_NAME' LIMIT 1")"
  [[ -n "$template_id" ]] || die "template '$CEO_TEMPLATE_NAME' not found"
  log "  template id: $template_id"
  local agent_name="CEO E2E $(date +%s)"
  local resp
  resp="$(api_post /api/agents/from-template "{\"template_id\":\"$template_id\",\"name\":\"$agent_name\",\"runtime_id\":\"$CLAUDE_RUNTIME_ID\",\"visibility\":\"workspace\"}")"
  CEO_AGENT_ID="$(printf '%s' "$resp" | jq -r '.agent.id // .id // empty')"
  [[ -n "$CEO_AGENT_ID" ]] || die "no agent id in response: $resp"
  log "  CEO agent: $CEO_AGENT_ID ($agent_name)"
  assert_exists "agent row" \
    "SELECT name FROM agent WHERE id='$CEO_AGENT_ID' AND runtime_id='$CLAUDE_RUNTIME_ID' AND archived_at IS NULL"
}

phase_trigger() {
  log "=== Phase 6: trigger issue (assign to CEO) ==="
  local desc
  desc=$(cat <<'DESC'
请用 multica-squads 组建一个小队,并用 multica-creating-agents 创建一个 worker 子 agent 加入小队。把"读 wiki 约定并产出 API 设计文档"的子任务派给该 worker。worker 先用 multica wiki search/read 读 wiki 约定,再用 multica oss upload 把 API 设计文档存到 projects/{project_id}/tasks/{task_id}/docs/api-design.md。
DESC
)
  local body
  body=$(jq -n --arg t "根据 wiki 约定为 e2e 测试模块写 API 设计文档,产出存 OSS" --arg d "$desc" \
    '{title:$t, description:$d, assignee_type:"agent", assignee_id:"'"$CEO_AGENT_ID"'", status:"todo", priority:"medium", allow_duplicate:true}')
  local resp
  resp="$(api_post /api/issues "$body")"
  ISSUE_ID="$(printf '%s' "$resp" | jq -r '.id // .issue.id // empty')"
  [[ -n "$ISSUE_ID" ]] || die "no issue id in response: $resp"
  log "  issue: $ISSUE_ID"
  # Give the scheduler a beat to enqueue the task.
  sleep 2
  # A regular task is enqueued for the assigned agent (is_leader_task=false,
  # squad_id=NULL). The squad + sub-agent form DURING CEO execution, not here.
  CEO_TASK_ID="$(psql_at "SELECT id FROM agent_task_queue WHERE issue_id='$ISSUE_ID' ORDER BY created_at DESC LIMIT 1")"
  [[ -n "$CEO_TASK_ID" ]] || die "no task enqueued for issue $ISSUE_ID"
  log "  task: $CEO_TASK_ID (squad/sub-agent form during CEO execution)"
}

# Global: track soft-check failures across Phase 7-8 (for the exit code).
CHECKS_FAILED=0

soft_count_ge() {
  local label="$1" expected="$2" sql="$3"
  local actual
  actual="$(psql_at "$sql")"; actual="${actual:-0}"
  if (( actual >= expected )); then
    log "  ✓ $label: $actual (>= $expected)"
  else
    log "  ✗ $label: expected >= $expected, got $actual"
    CHECKS_FAILED=1
  fi
}

poll_task_done() {
  # $1 = task id, $2 = timeout sec, $3 = label. Sets TASK_STATUS.
  local task_id="$1" deadline=$(( $(date +%s) + $2 )) label="$3"
  local status=""
  while [[ $(date +%s) -lt $deadline ]]; do
    status="$(psql_at "SELECT status FROM agent_task_queue WHERE id='$task_id'")"
    if [[ "$status" == "completed" || "$status" == "failed" || "$status" == "cancelled" ]]; then
      log "  $label: $status"
      TASK_STATUS="$status"
      return 0
    fi
    sleep 5
  done
  log "  ✗ $label: TIMEOUT (last status=$status)"
  TASK_STATUS="timeout"
  CHECKS_FAILED=1
}

phase_poll() {
  log "=== Phase 7: poll CEO task + observe squad/sub-agent ==="
  poll_task_done "$CEO_TASK_ID" 600 "CEO task"

  # Find the squad the CEO formed (leader=CEO, created after the CEO task started).
  SQUAD_ID="$(psql_at "SELECT id FROM squad WHERE leader_id='$CEO_AGENT_ID' AND created_at > (SELECT created_at FROM agent_task_queue WHERE id='$CEO_TASK_ID') ORDER BY created_at DESC LIMIT 1")"
  if [[ -n "$SQUAD_ID" ]]; then
    log "  squad: $SQUAD_ID"
    soft_count_ge "squad members" 1 \
      "SELECT count(*) FROM squad_member WHERE squad_id='$SQUAD_ID'"
  else
    log "  ✗ no squad found (leader=$CEO_AGENT_ID) — CEO may not have formed one"
    CHECKS_FAILED=1
  fi

  # Find the dynamically-created sub-agent (any agent created after the CEO task, excluding CEO itself).
  SUB_AGENT_ID="$(psql_at "SELECT a.id FROM agent a WHERE a.workspace_id='$WORKSPACE_ID' AND a.created_at > (SELECT created_at FROM agent_task_queue WHERE id='$CEO_TASK_ID') AND a.id != '$CEO_AGENT_ID' ORDER BY a.created_at DESC LIMIT 1")"
  if [[ -n "$SUB_AGENT_ID" ]]; then
    log "  sub-agent: $SUB_AGENT_ID"
    # Sub-agent existence (above) already proves the CEO delegated. This checks
    # that the CEO actively mentioned creating an agent or used CLI agent commands.
    soft_count_ge "multica agent creation evidence" 1 \
      "SELECT count(*) FROM task_message WHERE task_id='$CEO_TASK_ID' AND (content LIKE '%agent create%' OR content LIKE '%creating-agent%' OR input::text LIKE '%agent%')"
  else
    log "  ✗ no dynamically-created sub-agent found"
    CHECKS_FAILED=1
  fi

  log "=== Phase 8: poll sub-agent task + assert wiki/oss ==="
  # Find the sub-task: assigned to the sub-agent, or a child of the CEO task.
  SUB_TASK_ID="$(psql_at "SELECT id FROM agent_task_queue WHERE agent_id='$SUB_AGENT_ID' ORDER BY created_at DESC LIMIT 1")"
  if [[ -z "$SUB_TASK_ID" ]]; then
    SUB_TASK_ID="$(psql_at "SELECT id FROM agent_task_queue WHERE parent_task_id='$CEO_TASK_ID' AND is_leader_task=false ORDER BY created_at DESC LIMIT 1")"
  fi
  if [[ -n "$SUB_TASK_ID" ]]; then
    poll_task_done "$SUB_TASK_ID" 600 "sub-agent task"
    soft_count_ge "wiki reads" 1 \
      "SELECT count(*) FROM task_message WHERE task_id='$SUB_TASK_ID' AND (content LIKE '%wiki%' OR input::text LIKE '%wiki%')"
  else
    log "  ✗ no sub-task found (sub-agent=$SUB_AGENT_ID)"
    CHECKS_FAILED=1
  fi

  # Assert uploaded files on disk (local storage: multica oss upload → /api/upload-file
  # → server/data/uploads/...; oss_object is empty — no cloud OSS config in dev).
  local upload_dir="server/data/uploads/workspaces/${WORKSPACE_ID//-/}"  # local storage strips dashes from UUIDs
  local upload_count
  upload_count=$(find "$upload_dir" -name "*.md" -newer "$LOG_FILE" 2>/dev/null | wc -l) || true  # set -o pipefail: find exits 1 if dir missing
  if (( upload_count > 0 )); then
    log "  ✓ uploaded .md file(s): $upload_count (local storage: $upload_dir)"
  else
    log "  ✗ no uploaded .md files found in $upload_dir (newer than $LOG_FILE)"
    CHECKS_FAILED=1
  fi
}

phase_evidence() {
  log "=== Phase 9: write evidence report ==="
  {
    echo "# E2E Core Flow Report"
    echo
    echo "- Workspace: \`$WORKSPACE_ID\`"
    echo "- CEO agent: \`$CEO_AGENT_ID\`"
    echo "- Issue: \`$ISSUE_ID\`"
    echo "- Leader task: \`$CEO_TASK_ID\` (squad \`$SQUAD_ID\`)"
    echo "- Sub-agent: \`$SUB_AGENT_ID\` / task \`$SUB_TASK_ID\`"
    echo
    echo "## CEO task result"
    psql_at "SELECT result FROM agent_task_queue WHERE id='$CEO_TASK_ID'" 2>/dev/null | head -c 4000
    echo
    echo "## Sub-agent task result"
    psql_at "SELECT result FROM agent_task_queue WHERE id='$SUB_TASK_ID'" 2>/dev/null | head -c 4000
    echo
    echo "## Sub-agent tool-call sequence"
    psql_at "SELECT created_at::text || ' | ' || type || ' | ' || left(coalesce(content,''),120) FROM task_message WHERE task_id='$SUB_TASK_ID' ORDER BY seq" 2>/dev/null
    echo
    echo "## Local storage files"
    find "server/data/uploads/workspaces/${WORKSPACE_ID//-/}" -type f -newer "$LOG_FILE" 2>/dev/null | head -20
  } > "$REPORT_FILE"
  log "  report: $REPORT_FILE"
}

# ---------- Dispatcher ----------
run_phases() {
  local stop_after="$1"
  local phases=(preflight auth daemon oss wiki ceo trigger poll evidence)
  local p
  for p in "${phases[@]}"; do
    "phase_$p"
    [[ "$p" == "$stop_after" ]] && { log "=== stopped after $p ==="; return 0; }
  done
}

# ---------- Teardown ----------
CLEANUP=0
teardown() {
  local rc=$?
  # Kill the daemon we started (unless --keep-daemon).
  if [[ -n "$DAEMON_PID" && "${keep_daemon:-0}" == 0 ]]; then
    kill "$DAEMON_PID" 2>/dev/null || true
    log "daemon (pid $DAEMON_PID) stopped"
  fi
  # --cleanup: remove test data (reuses the existing 七牛华北 OSS config).
  if [[ "$CLEANUP" == "1" ]]; then
    log "=== cleanup: removing test data ==="
    psql_at "DELETE FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE title LIKE '%e2e 测试模块%')" 2>/dev/null || true
    psql_at "DELETE FROM issue WHERE title LIKE '%e2e 测试模块%'" 2>/dev/null || true
    psql_at "DELETE FROM agent WHERE name LIKE 'CEO E2E %'" 2>/dev/null || true
    log "  test data removed (agents + issues + tasks)"
  fi
  # On soft-check failures, write evidence + exit 2 (diagnosis mode).
  if [[ "${CHECKS_FAILED:-0}" != "0" ]]; then
    log "=== CHECKS_FAILED=$CHECKS_FAILED → exit 2 ==="
    phase_evidence 2>/dev/null || true
    exit 2
  fi
  log "=== exit $rc ==="
}

# Hard 30-min cap (spec §3): re-exec under timeout so the whole run is bounded.
# On SIGTERM → bash exits → the EXIT trap above fires to write the report.
if [[ -z "${E2E_REEXEC:-}" ]]; then
  E2E_REEXEC=1 exec timeout --signal=TERM 1800 bash "$0" "$@"
fi
main "$@"
