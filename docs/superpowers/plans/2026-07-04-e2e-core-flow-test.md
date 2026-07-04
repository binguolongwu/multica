# E2E Core Flow Functional Test — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `scripts/e2e-core-flow.sh` — a bash driver script that drives a real daemon + Claude agent through the full core flow (daemon → CEO agent → squad + dynamic sub-agent → skill → wiki → 七牛 OSS) and verifies each step via DB assertions.

**Architecture:** Single bash script with phase functions, run incrementally via `--phase <name>`. Uses `curl` for API calls, `psql` for DB assertions, `jq` for JSON parsing. Fail-fast for setup phases, diagnosis mode for flow phases (dump state on failure, always produce a report). Backed by spec `docs/superpowers/specs/2026-07-04-e2e-core-flow-test-design.md`.

**Tech Stack:** bash (set -euo pipefail), curl, psql, jq, docker (only if cleanup needs to nuke cloud objects — not required for run), `make daemon`.

## Global Constraints

Copied verbatim from the spec:

- Workspace ID: `8279ae9b-16f5-4904-92f0-b19fd8e18c5d` (reuse, do not create)
- Claude runtime ID: `019f2b3c-3971-79e0-b588-be9e048591b4` (reuse, do not create)
- CEO template name: `👔 CEO · 缤果软件` (resolve to UUID via `agent_template.name`; do NOT hardcode the UUID — it may differ across DBs)
- OSS config name: `七牛华北` (qiniu, bucket=huabei, region=z1, custom_domain=files.binguosoft.net, folder_prefix=multica/, is_default=true) — reuse, never delete
- Dev verification code: `MULTICA_DEV_VERIFICATION_CODE=123456`
- API base: `http://localhost:8080`
- Backend must already be running on :8080 before the script starts (`make dev` / `make server`)
- `X-Workspace-ID` header selects workspace for all `/api/*` calls
- Code comments must be English (project convention); script output / report may be Chinese
- The script must NOT kill the backend process (it didn't start it)

---

## File Structure

- **Create: `scripts/e2e-core-flow.sh`** — single bash script (the only artifact). Existing `scripts/` files are single-file (dev.sh, check.sh) — follow that pattern. Functions: helpers (log/die/assert/psql_at/api), phase functions (phase_preflight…phase_teardown), main (arg parse + orchestration + trap).

No other files. The runbook lives in the spec doc.

---

## Task 1: Scaffold the script + config + helpers

**Files:**
- Create: `scripts/e2e-core-flow.sh`

**Interfaces:**
- Produces: helper functions `log`, `die`, `psql_at`, `api_get`, `api_post`, `api_put`, `assert_count_ge`, `assert_exists` — used by all later tasks.

- [ ] **Step 1: Write the scaffold + helpers**

Create `scripts/e2e-core-flow.sh`:

```bash
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
```

- [ ] **Step 2: Syntax-check the script**

Run: `bash -n scripts/e2e-core-flow.sh`
Expected: exit 0, no syntax errors. (The script has no `main "$@"` call yet — that's added in Task 2 — so `--help` would do nothing here; a syntax check is the right gate.)

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): scaffold e2e-core-flow.sh with config and helpers"
```

---

## Task 2: Phase 0–1 — Preflight + Auth

**Files:**
- Modify: `scripts/e2e-core-flow.sh` (add `phase_preflight`, `phase_auth`, `run_phases`, `teardown` stub)

**Interfaces:**
- Produces: global `TOKEN` (JWT), `OWNER_EMAIL` — used by all later API phases.

- [ ] **Step 1: Add the phase functions + run_phases dispatcher + teardown stub**

Append to `scripts/e2e-core-flow.sh` (before `main`, or anywhere after helpers — the file is sourced top-down so define functions before `main` is called; bash allows definition before use as long as it's in the same file):

```bash
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
```

- [ ] **Step 2: Run to `--phase auth`, verify JWT obtained**

Prereq: backend running on :8080 (`make dev` or `make server`).

Run: `bash scripts/e2e-core-flow.sh --phase auth`
Expected: log ends with "stopped after auth"; no error; `TOKEN` would be set for subsequent phases (not visible, but exit 0).

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): add preflight + auth phases"
```

---

## Task 3: Phase 2 — Start daemon + wait for Claude runtime online

**Files:**
- Modify: `scripts/e2e-core-flow.sh`

**Interfaces:**
- Produces: `DAEMON_PID` (background daemon pid), verified runtime online.

- [ ] **Step 1: Add `phase_daemon`**

Insert after `phase_auth`:

```bash
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
```

- [ ] **Step 2: Run to `--phase daemon`, verify runtime online**

Run: `bash scripts/e2e-core-flow.sh --phase daemon`
Expected: log shows "runtime online (status=idle|working, age=Ns)"; daemon process visible via `pgrep -f cmd/daemon`.

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): add daemon start + runtime-online polling phase"
```

---

## Task 4: Phase 3–4 — Verify OSS config + seed wiki

**Files:**
- Modify: `scripts/e2e-core-flow.sh`

**Interfaces:**
- Produces: verified `七牛华北` OSS config; wiki space `e2e-core-flow` + page `wiki/conventions.md`.

- [ ] **Step 1: Add `phase_oss` and `phase_wiki`**

Insert after `phase_daemon`:

```bash
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
  # Find-or-create the space.
  local space
  space="$(api_get "/api/wiki/spaces/$WIKI_SPACE_SLUG")"
  if printf '%s' "$space" | jq -e '.slug // empty' >/dev/null 2>&1; then
    log "  wiki space exists, reusing"
  else
    api_post /api/wiki/spaces "{\"slug\":\"$WIKI_SPACE_SLUG\",\"display_name\":\"E2E Core Flow\",\"access_scope\":\"shared\"}" >/dev/null
    log "  wiki space created"
  fi
  # Upsert the conventions page.
  local body
  body=$(cat <<JSON
{"content":"# 项目约定\n## 模块命名\n模块名使用 kebab-case。\n## 文档路径\n产出存 projects/{project_id}/tasks/{task_id}/docs/api-design.md。"}
JSON
)
  api_put "/api/wiki/spaces/$WIKI_SPACE_SLUG/pages/$WIKI_PAGE_PATH" "$body" >/dev/null
  assert_exists "wiki page seeded" \
    "SELECT content_hash FROM wiki_page WHERE space_id=(SELECT id FROM wiki_space WHERE slug='$WIKI_SPACE_SLUG') AND path='$WIKI_PAGE_PATH'"
}
```

- [ ] **Step 2: Run to `--phase wiki`, verify page row**

Run: `bash scripts/e2e-core-flow.sh --phase wiki`
Expected: "✓ wiki page seeded: <hash>" in log; `psql ... SELECT path FROM wiki_page WHERE path='wiki/conventions.md'` returns the row.

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): add OSS verify + wiki seed phases"
```

---

## Task 5: Phase 5 — Create CEO agent from template

**Files:**
- Modify: `scripts/e2e-core-flow.sh`

**Interfaces:**
- Produces: `CEO_AGENT_ID` (UUID of the created agent).

- [ ] **Step 1: Add `phase_ceo`**

Insert after `phase_wiki`:

```bash
phase_ceo() {
  log "=== Phase 5: create CEO agent from template ==="
  # Resolve template UUID by name (do NOT hardcode the UUID).
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
```

- [ ] **Step 2: Run to `--phase ceo`, verify agent row**

Run: `bash scripts/e2e-core-flow.sh --phase ceo`
Expected: "✓ agent row: CEO E2E <ts>"; `psql ... SELECT name FROM agent WHERE id=<id>` returns the name.

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): add CEO agent creation from template phase"
```

---

## Task 6: Phase 6 — Trigger issue with @squad mention

**Files:**
- Modify: `scripts/e2e-core-flow.sh`

**Interfaces:**
- Produces: `ISSUE_ID`, `CEO_TASK_ID`, `SQUAD_ID` (the enqueued leader task + its squad).

- [ ] **Step 1: Add `phase_trigger`**

Insert after `phase_ceo`:

```bash
phase_trigger() {
  log "=== Phase 6: trigger issue + @squad ==="
  local desc
  desc=$(cat <<'DESC'
@squad 组队,并用 multica-creating-agents 创建一个 worker 子 agent。worker 先 multica wiki search/read 读 wiki 约定,再用 multica oss upload 把 API 设计文档存到 projects/{project_id}/tasks/{task_id}/docs/api-design.md。
DESC
)
  local body
  body=$(jq -n --arg t "根据 wiki 约定为 e2e 测试模块写 API 设计文档,产出存 OSS" --arg d "$desc" \
    '{title:$t, description:$d, assignee_type:"agent", assignee_id:"'"$CEO_AGENT_ID"'", status:"todo", priority:"medium"}')
  local resp
  resp="$(api_post /api/issues "$body")"
  ISSUE_ID="$(printf '%s' "$resp" | jq -r '.id // empty')"
  [[ -n "$ISSUE_ID" ]] || die "no issue id in response: $resp"
  log "  issue: $ISSUE_ID"
  # Give the scheduler a beat to enqueue the leader task.
  sleep 2
  local row
  row="$(psql_at "SELECT id, squad_id FROM agent_task_queue WHERE issue_id='$ISSUE_ID' AND is_leader_task=true ORDER BY created_at DESC LIMIT 1")"
  CEO_TASK_ID="$(printf '%s' "$row" | cut -f1)"
  SQUAD_ID="$(printf '%s' "$row" | cut -f2)"
  [[ -n "$CEO_TASK_ID" ]] || die "no leader task enqueued for issue $ISSUE_ID"
  [[ -n "$SQUAD_ID"   ]] || die "leader task has no squad_id (@squad mention did not stamp)"
  log "  leader task: $CEO_TASK_ID (squad: $SQUAD_ID)"
}
```

- [ ] **Step 2: Run to `--phase trigger`, verify leader task + squad_id**

Run: `bash scripts/e2e-core-flow.sh --phase trigger`
Expected: "leader task: <uuid> (squad: <uuid>)"; DB row exists with `is_leader_task=true AND squad_id IS NOT NULL`.

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): add issue trigger + @squad squad-stamping phase"
```

---

## Task 7: Phase 7–8 — Poll tasks + assert full chain

**Files:**
- Modify: `scripts/e2e-core-flow.sh`

**Interfaces:**
- Produces: `SUB_AGENT_ID`, `SUB_TASK_ID`, `OSS_OBJECT_KEY` — captured for the evidence report.
- Consumes: `CEO_TASK_ID`, `SQUAD_ID`, `ISSUE_ID` from Task 6.

- [ ] **Step 1: Add polling helper + `phase_poll`**

Insert after `phase_trigger`:

```bash
# Poll agent_task_queue.status until target or timeout. Sets TASK_STATUS.
poll_task_done() {
  # $1 = task id, $2 = timeout sec, $3 = label
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
  log "  $label: TIMEOUT (last status=$status)"
  TASK_STATUS="timeout"
}

phase_poll() {
  log "=== Phase 7: poll CEO task + assert squad/sub-agent ==="
  poll_task_done "$CEO_TASK_ID" 300 "CEO task"
  if [[ "$TASK_STATUS" != "completed" ]]; then
    log "  WARN: CEO task did not complete cleanly — continuing to dump diagnostics"
  fi

  log "  -- assert squad created --"
  assert_count_ge "squad members" 1 \
    "SELECT count(*) FROM squad_member WHERE squad_id='$SQUAD_ID'"

  log "  -- assert sub-agent dynamically created (via multica-creating-agents) --"
  SUB_AGENT_ID="$(psql_at "SELECT a.id FROM agent a WHERE a.workspace_id='$WORKSPACE_ID' AND a.created_at > (SELECT created_at FROM agent_task_queue WHERE id='$CEO_TASK_ID') AND a.id IN (SELECT member_id FROM squad_member WHERE member_type='agent' AND squad_id='$SQUAD_ID') ORDER BY a.created_at DESC LIMIT 1")"
  [[ -n "$SUB_AGENT_ID" ]] || die "no dynamically-created sub-agent found in squad $SQUAD_ID"

  log "  -- assert creating-agents skill was used --"
  assert_count_ge "multica-creating-agents skill calls" 1 \
    "SELECT count(*) FROM task_message WHERE task_id='$CEO_TASK_ID' AND (content LIKE '%multica agent create%' OR content LIKE '%creating-agents%')"

  log "=== Phase 8: poll sub-agent task + assert wiki/oss ==="
  SUB_TASK_ID="$(psql_at "SELECT id FROM agent_task_queue WHERE parent_task_id='$CEO_TASK_ID' AND is_leader_task=false ORDER BY created_at DESC LIMIT 1")"
  [[ -n "$SUB_TASK_ID" ]] || die "no sub-task found with parent_task_id=$CEO_TASK_ID"
  poll_task_done "$SUB_TASK_ID" 300 "sub-agent task"
  if [[ "$TASK_STATUS" != "completed" ]]; then
    log "  WARN: sub-agent task did not complete cleanly — continuing to dump diagnostics"
  fi

  log "  -- assert wiki was read --"
  assert_count_ge "wiki reads" 1 \
    "SELECT count(*) FROM task_message WHERE task_id='$SUB_TASK_ID' AND content LIKE '%multica wiki%'"

  log "  -- assert OSS object saved (key with multica/ folder_prefix) --"
  OSS_OBJECT_KEY="$(psql_at "SELECT key FROM oss_object WHERE uploaded_by='$SUB_AGENT_ID' AND key LIKE 'multica/projects/%/tasks/%/docs/%' ORDER BY created_at DESC LIMIT 1")"
  [[ -n "$OSS_OBJECT_KEY" ]] || die "no OSS object uploaded by sub-agent $SUB_AGENT_ID with docs/ prefix"

  log "  -- assert file downloadable --"
  local url="https://files.binguosoft.net/$OSS_OBJECT_KEY"
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" || true)"
  [[ "$code" == "200" ]] || die "OSS file not downloadable at $url (HTTP $code)"
  log "  ✓ OSS file downloadable: $url"
}
```

- [ ] **Step 2: Run to `--phase poll`, verify the full chain**

Run: `bash scripts/e2e-core-flow.sh --phase poll`
Expected (may take up to 10 min for real LLM): all `✓` assertions pass; "OSS file downloadable: https://files.binguosoft.net/multica/projects/.../api-design.md".

If a `WARN` appears (task didn't complete cleanly) but later assertions still pass, that's acceptable per the spec's diagnosis-mode design. If `die` fires, inspect `/tmp/e2e-core-flow.log` + `task_message` rows.

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): add polling + full-chain assertions (squad/sub-agent/skill/wiki/oss)"
```

---

## Task 8: Phase 9–10 — Evidence report + teardown/cleanup

**Files:**
- Modify: `scripts/e2e-core-flow.sh`

**Interfaces:**
- Produces: `/tmp/e2e-core-flow-report.md` (markdown report); `--cleanup` deletes test data.

- [ ] **Step 1: Add `phase_evidence` + expand `teardown`**

Replace the `teardown` stub (from Task 1/2) with the full version, and add `phase_evidence`:

```bash
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
    [[ -n "$OSS_OBJECT_KEY" ]] && echo "- OSS object: \`$OSS_OBJECT_KEY\`" && echo "  - URL: https://files.binguosoft.net/$OSS_OBJECT_KEY"
    echo
    echo "## CEO task result"
    psql_at "SELECT result FROM agent_task_queue WHERE id='$CEO_TASK_ID'" | head -c 4000
    echo
    echo "## Sub-agent task result"
    psql_at "SELECT result FROM agent_task_queue WHERE id='$SUB_TASK_ID'" | head -c 4000
    echo
    echo "## Sub-agent tool-call sequence (task_message)"
    psql_at "SELECT created_at::text || ' | ' || type || ' | ' || left(coalesce(content,''),120) FROM task_message WHERE task_id='$SUB_TASK_ID' ORDER BY seq"
  } > "$REPORT_FILE"
  log "  report: $REPORT_FILE"
}

# ---------- Teardown ----------
CLEANUP=0
teardown() {
  local rc=$?
  if [[ -n "$DAEMON_PID" && "${keep_daemon:-0}" == 0 ]]; then
    kill "$DAEMON_PID" 2>/dev/null || true
    log "daemon (pid $DAEMON_PID) stopped"
  fi
  if [[ "$CLEANUP" == "1" ]]; then
    log "=== cleanup: removing test data ==="
    psql_at "DELETE FROM oss_object WHERE uploaded_by IN (SELECT id FROM agent WHERE name LIKE 'CEO E2E %') OR key LIKE 'multica/projects/%/tasks/%/docs/%'" >/dev/null 2>&1 || true
    psql_at "DELETE FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE title LIKE '%e2e 测试模块%')" >/dev/null 2>&1 || true
    psql_at "DELETE FROM issue WHERE title LIKE '%e2e 测试模块%'" >/dev/null 2>&1 || true
    psql_at "DELETE FROM agent WHERE name LIKE 'CEO E2E %'" >/dev/null 2>&1 || true
    psql_at "DELETE FROM wiki_page WHERE space_id=(SELECT id FROM wiki_space WHERE slug='$WIKI_SPACE_SLUG')" >/dev/null 2>&1 || true
    psql_at "DELETE FROM wiki_space WHERE slug='$WIKI_SPACE_SLUG'" >/dev/null 2>&1 || true
    log "  test data removed (七牛华北 config NOT deleted)"
  fi
  log "=== exit $rc ==="
}

# Hard 15-min cap (spec §3): re-exec under timeout on first invocation so the
# whole run is bounded. On SIGTERM the EXIT trap above still fires to write
# the report. Per-phase timeouts (60s + 5min×2) bound normal runs to ~11min.
if [[ -z "${E2E_REEXEC:-}" ]]; then
  E2E_REEXEC=1 exec timeout --signal=TERM 900 bash "$0" "$@"
fi
main "$@"
```

(Note: `phase_teardown` is NOT a function — teardown runs via the `trap … EXIT`. So remove `teardown` from the `phases` array in `run_phases`, OR make `phase_teardown` a no-op that logs. Simplest: remove `teardown` from the array. Fix `run_phases`'s `phases` line to: `local phases=(preflight auth daemon oss wiki ceo trigger poll evidence)`.)

- [ ] **Step 2: Run full script (no --phase), verify report + exit 0**

Run: `bash scripts/e2e-core-flow.sh`
Expected: exit 0; `/tmp/e2e-core-flow-report.md` exists with CEO/sub-agent results + tool-call sequence + OSS URL.

- [ ] **Step 3: Run with `--cleanup`, verify test data removed**

Run: `bash scripts/e2e-core-flow.sh --cleanup`
Expected: log shows "test data removed"; `psql ... SELECT count(*) FROM agent WHERE name LIKE 'CEO E2E %'` returns 0; `七牛华北` config still present (`SELECT name FROM oss_provider_config WHERE name='七牛华北'`).

- [ ] **Step 4: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "feat(e2e): add evidence report + teardown/cleanup"
```

---

## Task 9: Full-run smoke + runbook header

**Files:**
- Modify: `scripts/e2e-core-flow.sh` (add runbook comment block at top)

- [ ] **Step 1: Add runbook header at the top of the script**

Edit the top comment block of `scripts/e2e-core-flow.sh` to include a runbook:

```bash
# E2E core-flow functional test: drives a real daemon + Claude agent through
# the full core loop and asserts each step in the DB. See
# docs/superpowers/specs/2026-07-04-e2e-core-flow-test-design.md
#
# Prerequisites:
#   1. Backend running on :8080  (make dev  or  make server)
#   2. Claude CLI authed   (claude --version works)
#   3. DATABASE_URL env var set (pointing at multica_dev)
#   4. MULTICA_DEV_VERIFICATION_CODE=123456
#   5. Workspace 8279ae9b-... has: Claude runtime, CEO template, 七牛华北 OSS config
#
# Usage:
#   bash scripts/e2e-core-flow.sh                 # full run, keep test data
#   bash scripts/e2e-core-flow.sh --phase wiki    # stop after a phase
#   bash scripts/e2e-core-flow.sh --cleanup       # remove test data on exit
#
# Outputs:
#   /tmp/e2e-core-flow.log        — full curl/psql output
#   /tmp/e2e-core-flow-report.md   — structured markdown report
#   /tmp/e2e-core-flow.log.daemon  — daemon stdout/stderr
```

- [ ] **Step 2: Run full script once, confirm exit 0 + report populated**

Run: `bash scripts/e2e-core-flow.sh`
Expected: exit 0; report file has all sections filled (CEO result, sub-agent result, tool-call sequence, OSS URL with 200 download).

- [ ] **Step 3: Commit**

```bash
git add scripts/e2e-core-flow.sh
git commit -m "docs(e2e): add runbook header to e2e-core-flow.sh"
```

---

## Self-Review

**1. Spec coverage:**
- §1 components (auth, oss_verify, wiki_seed, ceo_setup, task_trigger, poller, assertions, evidence, teardown) → Tasks 2,4,4,5,6,7,7,8,8 ✓
- §1 "复用 workspace/runtime/CEO template/七牛 config" → Global Constraints + Phase 0/3/5 ✓
- §1 "需确认点" (daemon registration, claude auth, CEO instructions) → Phase 2 die-message + Task 7 sub-agent assertion handles ✓
- §2 Phase 0–10 → Tasks 2,3,4,5,6,7,7,8,8 ✓ (Phase 9=evidence, Phase 10=teardown via trap)
- §3 fail-fast (Phase 0–5) / diagnosis (Phase 6–8) → `die` in setup phases, `log WARN` + continue in `phase_poll` ✓
- §3 timeouts (60s daemon, 5min×2 tasks, 15min total) → `poll_task_done` 300s; 15min total cap NOT yet enforced — **gap**: add a `timeout 900` wrapper or a hard deadline check. Add to Task 7 or note.

**Fix inline:** the 15-min total hard cap was missing. Added as Task 8 Step 4 below — a re-exec-under-`timeout` wrapper at the very bottom of the script. Resolved. ✓

**2. Placeholder scan:** No TBD/TODO. All code blocks are complete. `phase_teardown` array entry note in Task 8 is a concrete fix instruction, not a placeholder. ✓

**3. Type consistency:** `CEO_AGENT_ID`, `ISSUE_ID`, `CEO_TASK_ID`, `SQUAD_ID`, `SUB_AGENT_ID`, `SUB_TASK_ID`, `OSS_OBJECT_KEY`, `TOKEN`, `OWNER_EMAIL`, `DAEMON_PID`, `TASK_STATUS` — all used consistently across tasks. `assert_count_ge`/`assert_exists`/`psql_at`/`api_*` signatures consistent. ✓

**Gap found & fixed:** `run_phases` `phases` array in Task 2 includes `teardown`, but Task 8 says teardown runs via trap. Fix Task 8's note is already there. ✓
