-- 137_skill_type_system

-- ============================================================================
-- 1. Add skill_type column
-- ============================================================================
ALTER TABLE skill ADD COLUMN IF NOT EXISTS skill_type TEXT;
UPDATE skill SET skill_type = 'workspace' WHERE skill_type IS NULL;
ALTER TABLE skill ALTER COLUMN skill_type SET NOT NULL;
ALTER TABLE skill ADD CONSTRAINT ck_skill_type CHECK (skill_type IN ('builtin', 'platform', 'workspace'));

-- ============================================================================
-- 2. Make workspace_id nullable
-- ============================================================================
ALTER TABLE skill ALTER COLUMN workspace_id DROP NOT NULL;

-- ============================================================================
-- 3. Enforce workspace_id not null for workspace type
-- ============================================================================
ALTER TABLE skill ADD CONSTRAINT ck_skill_workspace_required CHECK (
    (skill_type = 'workspace' AND workspace_id IS NOT NULL)
    OR (skill_type IN ('builtin', 'platform') AND workspace_id IS NULL)
);

-- ============================================================================
-- 4. Rebuild unique index (PG15+ NULLS NOT DISTINCT)
-- ============================================================================
ALTER TABLE skill DROP CONSTRAINT IF EXISTS skill_workspace_id_name_key;
CREATE UNIQUE INDEX idx_skill_unique_name ON skill (workspace_id, name) NULLS NOT DISTINCT;

-- ============================================================================
-- 5. Rename agent_template.skill_urls to skill_ids
-- ============================================================================
ALTER TABLE agent_template RENAME COLUMN skill_urls TO skill_ids;

-- ============================================================================
-- 6. Seed built-in skills
-- ============================================================================

-- multica-autopilots
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-autopilots',
 'Use when creating, updating, inspecting, triggering, or debugging Multica autopilots. Covers the full chain: schedule/webhook/manual trigger, create_issue vs run_only execution, agent/squad leader admission, runs, created issues/tasks, webhook URL rotation, and side-effect boundaries.',
 $skill1$---
name: multica-autopilots
description: "Use when creating, updating, inspecting, triggering, or debugging Multica autopilots. Covers the full chain: schedule/webhook/manual trigger, create_issue vs run_only execution, agent/squad leader admission, runs, created issues/tasks, webhook URL rotation, and side-effect boundaries."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica Autopilots

## Quick start

Autopilots are durable automations. Read before mutating:

```bash
multica autopilot list --output json
multica autopilot get <autopilot-id> --output json
multica autopilot runs <autopilot-id> --output json
```

Do not run `trigger`, `delete`, `trigger-delete`, or `trigger-rotate-url` to test. Those are real side effects.

## Core model

An autopilot is not an agent. It is a rule that dispatches work to an agent, or to a squad's leader agent.

The chain is: trigger fires (`schedule`, `webhook`, or `manual`) -> `autopilot_run` row -> `execution_mode` decides output -> assignee readiness check -> issue/task execution -> run status sync.

Execution modes:

- `create_issue` creates a Multica issue, making the run visible as issue state.
- `run_only` creates an agent task directly. No issue is created; any durable
  report location has to come from other task context or instructions.

`issue-title-template` only supports `{{date}}`. Do not invent `{{trigger_id}}`, `{{branch}}`, or other variables.

## CLI

```bash
multica autopilot list --output json
multica autopilot get <autopilot-id> --output json
multica autopilot create --title "<title>" --description "<task prompt>" --agent <agent-name-or-id> --mode create_issue|run_only --output json
multica autopilot update <autopilot-id> --status active|paused --output json
multica autopilot runs <autopilot-id> --output json
multica autopilot trigger-add <autopilot-id> --kind schedule --cron "0 9 * * *" --timezone Asia/Shanghai --output json
multica autopilot trigger-add <autopilot-id> --kind webhook --label "ci" --output json
multica autopilot trigger <autopilot-id> --output json
multica autopilot trigger-rotate-url <autopilot-id> <trigger-id> --yes --output json
```

Use `trigger` only when the user explicitly asks for a manual run. Use `trigger-rotate-url` only when rotating a webhook URL; the old URL stops being valid.

Webhook trigger output can include a URL/token. Do not paste webhook tokens or signing material into comments, logs, docs, or PRs. Redact secrets.

## Debugging

For "why didn't it run":

1. `multica autopilot get <id> --output json` — status, mode, assignee, triggers.
2. `multica autopilot runs <id> --output json` — run status and failure reason.
3. If assigned to a squad, inspect the squad: `multica squad get <squad-id> --output json`; execution goes to the leader.
4. Inspect the target agent/runtime: `multica agent get <agent-id> --output json` and `multica runtime list --output json`.
5. For `create_issue`, inspect the created issue if the run records one.

## Side effects

These mutate durable state or start work: `create`, `update`, `delete`, trigger add/update/delete/rotate, `trigger`, and webhook calls to `/api/webhooks/autopilots/{token}`.

More source-backed details: `references/autopilots-source-map.md`.
$skill1$,
 '{"origin": {"type": "builtin"}}');

-- multica-autopilots: references/autopilots-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-autopilots'),
 'references/autopilots-source-map.md',
 $file1$# Autopilots source map

- `server/cmd/multica/cmd_autopilot.go` registers `list`, `get`, `create`, `update`, `delete`, `trigger`, `runs`, `trigger-add`, `trigger-update`, `trigger-delete`, and `trigger-rotate-url`.
- The CLI maps reads/writes to `/api/autopilots`, `/api/autopilots/{id}`, `/api/autopilots/{id}/trigger`, `/api/autopilots/{id}/runs`, and trigger subroutes.
- `server/internal/service/autopilot.go` has `DispatchAutopilot`, creates `autopilot_run`, and switches on `execution_mode`.
- `create_issue` calls `dispatchCreateIssue`; `run_only` calls `dispatchRunOnly`.
- `resolveAutopilotLeader` resolves squad-assigned autopilots to the squad leader.
- `AgentReadiness` blocks archived/runtime-unready agents before enqueue.
- `server/cmd/server/router.go` exposes authenticated `/api/autopilots` routes and unauthenticated webhook ingress `/api/webhooks/autopilots/{token}`.
$file1$);

-- multica-creating-agents
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-creating-agents',
 'Use when creating, inspecting, or debugging a Multica agent through the `multica agent` CLI or `POST /api/agents` — what each field is, its persisted shape, whether it is metadata-only or consumed by the daemon at claim time, which inputs are validated/rejected, how custom_env secrets are gated, and how skill binding behaves. Not for assigning issues to existing agents or for runtime task prompts.',
 $skill2$---
name: multica-creating-agents
description: "Use when creating, inspecting, or debugging a Multica agent through the `multica agent` CLI or `POST /api/agents` — what each field is, its persisted shape, whether it is metadata-only or consumed by the daemon at claim time, which inputs are validated/rejected, how custom_env secrets are gated, and how skill binding behaves. Not for assigning issues to existing agents or for runtime task prompts."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Creating Multica agents

This is the contract for Multica's agent-creation path: what the create entry
points accept, what the server validates and rejects, how each field is
persisted, and which fields the daemon actually reads at claim time. It is
not a parameter manual — it states source-traced facts, and every claim is
backed by `file:line` in `references/creating-agents-source-map.md`.

## Quick start (read-only inspection)

These commands read state and have no side effects:

```bash
multica agent get <agent-id> --output json      # full persisted agent record
multica agent skills list <agent-id> --output json   # current skill bindings
multica agent env get <agent-id> --output json  # plaintext env (owner/admin only, agents denied)
```

`agent get` returns the persisted agent including `runtime_id`, `model`,
`thinking_level`, `custom_args`, `has_custom_env`, `custom_env_key_count`, and
`skills`. It never returns plaintext `custom_env`.

## Core model

An agent is a workspace-scoped row (table `agent`). Creation is a single
`POST /api/agents` (`multica agent create`). At task claim time the daemon
re-reads the agent row and assembles the runtime payload — so the persisted
fields, not the create-time output, are what the agent runs on.

Two distinct text fields, often confused:

- `description` is a catalog summary. It is stored and shown in listings; the
  daemon does NOT inject it into the agent's runtime prompt. Treat it as
  human-facing metadata only. Capped at 255 Unicode code points.
- `instructions` is the runtime behavior contract. The daemon reads it at
  claim time and ships it to the provider as the agent's durable instructions.
  Persona, responsibilities, boundaries, output and escalation rules go here,
  not in `description`.

## CLI / API entry points

Minimum create call (`--name` and `--runtime-id` are both required):

```bash
multica agent create --name <name> --runtime-id <runtime-id> \
  --description "<short catalog summary>" \
  --instructions "<runtime behavior contract>" \
  --output json
```

`runAgentCreate` builds a JSON body and posts it to `/api/agents`. It only
adds a key when its flag was provided — `description`/`instructions` on a
non-empty value, the rest (`runtime-config`, `custom-args`, `model`,
`thinking-level`, `visibility`, …) on the flag being `Changed` — so omitted
flags fall through to server defaults rather than sending empty strings.

The HTTP body (`CreateAgentRequest`) accepts: `name`, `description`,
`instructions`, `runtime_id`, `runtime_config`, `custom_env`, `custom_args`,
`model`, `thinking_level`, `visibility`, `max_concurrent_tasks`, `mcp_config`.

## Field contracts

| Field | Persisted as | Validated? | Consumed by |
|---|---|---|---|
| `name` | `agent.name` | required, 400 if empty | listings, runtime payload |
| `description` | `agent.description` | 400 if > 255 code points | catalog/listing only — NOT the runtime prompt |
| `instructions` | `agent.instructions` | none | daemon → provider at claim time |
| `runtime_id` | `agent.runtime_id` | required (400) + must resolve to a runtime in this workspace | selects runtime/provider |
| `model` | `agent.model` (nullable) | none beyond runtime support | daemon reads; empty = runtime default |
| `thinking_level` | `agent.thinking_level` (nullable) | provider-level enum; unknown literal → 400 | daemon; empty = runtime default |
| `custom_args` | `agent.custom_args` (JSON array) | JSON shape checked CLI-side; server stores as-is | daemon (extra CLI switches); defaults to `[]` |
| `runtime_config` | `agent.runtime_config` (JSON) | JSON shape checked CLI-side; server stores as-is | runtime-specific config; defaults to `{}` |
| `custom_env` | `agent.custom_env` (JSON object) | — | daemon (process env); see Env & secrets |
| `mcp_config` | `agent.mcp_config` (raw JSON) | CLI checks it is a JSON object or `null`; server stores as-is. At create, literal `null` is dropped (no-op); at update, `null` clears the column | daemon → provider (MCP servers) — **runtime-consumed**; redacted on read |
| `visibility` | `agent.visibility` | — | access control; defaults to `private`; gates who can read/route a private agent (e.g. a private squad leader) — NOT the runtime prompt |
| `max_concurrent_tasks` | `agent.max_concurrent_tasks` | — | scheduler task cap; defaults to `6` |

Defaults when omitted: `runtime_config` → `{}`, `custom_env` → `{}`,
`custom_args` → `[]`, `visibility` → `private`, `max_concurrent_tasks` → `6`
(all materialized server-side before the insert). `custom_args`/`runtime_config`
are typed `[]string`/`any` and marshaled as-is — the JSON-shape rejection
happens in the CLI, not the create handler.

`thinking_level` is validated only at the provider level: an unrecognized
literal returns 400, but a value that is valid for the provider yet
unsupported for the chosen model is NOT rejected here — that gap surfaces as a
daemon-side task error at execution time.

Set it from the CLI with `--thinking-level` on `agent create` and `agent
update`, mirroring `--model`: the flag is a thin pass-through to the top-level
`thinking_level` field, and on update an empty string (`--thinking-level ""`)
clears it back to the runtime default. The CLI deliberately does not enumerate
the valid levels — they are runtime/model-specific (Claude
`low|medium|high|xhigh|max`, Codex `none|minimal|low|medium|high|xhigh`, and
others), so it forwards whatever you pass and lets the server's provider
catalog accept or reject it. A runtime whose provider has no thinking concept
rejects any non-empty value with a 400.

### model vs custom_args

`model` is a first-class persisted column the daemon reads directly.
`custom_args` are raw provider CLI args. The CLI help notes that some providers
(codex app-server, openclaw) reject `--model` inside `custom_args` — but that is
documented CLI guidance, not a server-enforced invariant; nothing in the create
handler inspects `custom_args` for a model flag.

## Env & secrets

`custom_env` is secret material. The CLI offers three input channels; two keep
secrets out of shell history and the process list:

```bash
multica agent create --name <name> --runtime-id <runtime-id> --custom-env-stdin --output json
multica agent create --name <name> --runtime-id <runtime-id> --custom-env-file <0600-json> --output json
```

`--custom-env-stdin` reads the JSON object from stdin; `--custom-env-file`
reads it from a file (suggested mode 0600). The third channel,
`--custom-env <json>`, puts the value on the command line where shell history
and `ps` can see it — avoid it for real secrets.

Read-side facts (these are the wrong assumptions to avoid):

- Agent resources never expose plaintext `custom_env`. `agent
  list/get/create/update` and WS events return only `has_custom_env` (bool) and
  `custom_env_key_count` (int).
- Reading plaintext values requires the dedicated `GET /api/agents/{id}/env`
  endpoint (`multica agent env get`). It is gated to workspace **owner/admin**
  members, and **agent actors are denied** regardless of the backing member's
  role — a running agent cannot read another agent's secrets.
- Writing values after creation does NOT go through `agent update`. The generic
  update handler rejects any `custom_env` field with a 400 ("use PUT
  /api/agents/{id}/env"). Plaintext env writes are handled by
  `PUT /api/agents/{id}/env` (`multica agent env set`), which is owner/admin-only
  and writes an audit row.

### mcp_config

`mcp_config` is the agent's MCP server configuration (a JSON object such as
`{"mcpServers": {…}}`). It is also secret material — MCP entries routinely embed
API tokens — and offers the same three input channels as `custom_env`, on BOTH
`agent create` and `agent update`:

```bash
multica agent create --name <name> --runtime-id <runtime-id> --mcp-config-file <0600-json> --output json
multica agent update <agent-id> --mcp-config-stdin --output json
multica agent update <agent-id> --mcp-config 'null'   # clears the config
```

`--mcp-config-stdin` / `--mcp-config-file` keep the value out of shell history
and `ps`; the inline `--mcp-config <json>` does not. The CLI requires a JSON
**object** or the literal `null`; a top-level array or primitive is rejected
client-side, and empty stdin/file input errors rather than silently clearing.

Two ways `mcp_config` differs from `custom_env`:

- **It IS settable through `agent update`.** Unlike `custom_env`, `mcp_config`
  has no dedicated audited endpoint — the generic `PUT /api/agents/{id}` accepts
  it. Tri-state per the raw request body: field omitted → no change; `null` →
  clear; object → replace.
- **It is serialized on read, but redacted.** `agent get`/`list` return
  `mcp_config` only to callers allowed to view agent secrets; otherwise the
  field is `null` and `mcp_config_redacted` is `true`. Agent actors never see
  it, and a workspace may force redaction for everyone.

## Skill binding

Creating an agent does NOT bind any workspace skill — binding is a separate
call after the agent exists. Two distinct verbs:

- `add` is additive — it merges the given ids with existing bindings
  (`POST /api/agents/{id}/skills/add`).
- `set` is replace-all — it overwrites the entire binding list with exactly
  the given ids (`PUT /api/agents/{id}/skills`); `--skill-ids ''` clears all.

```bash
multica agent skills add <agent-id> --skill-ids <skill-id> --output json
multica agent skills list <agent-id> --output json
```

At claim time the daemon assembles the agent's skills as workspace-bound skills
FIRST, then appends the platform built-in skills. `LoadAgentSkills` loads each
bound skill's content plus its supporting files; built-in skills are embedded
at compile time and loaded from `SKILL.md` + sibling files. Both reach the
provider as skill content — which is why capability belongs in a bound skill,
not pasted into `instructions`.

## Side effects needing approval

Read-only (safe): `agent get`, `agent skills list`, `agent env get`.

State-changing (require an explicit instruction — do not run speculatively):

- `multica agent create` — inserts a new agent row.
- `multica agent skills add` / `set` — mutate bindings (`set` is destructive:
  it drops bindings not in the new list).
- `multica agent env set` — overwrites the full `custom_env` map and writes an
  audit row.

## Common wrong assumptions

- "`description` is the prompt." It is not — only `instructions` reaches the
  runtime. A rich description with empty instructions yields a named shell with
  no operating contract.
- "Create binds the agent's skills." It does not; bind explicitly afterward.
- "`agent update` can rotate env." It cannot — it 400s on `custom_env`; use the
  env endpoint.
- "`mcp_config` behaves like `custom_env` on update." It does not — `mcp_config`
  IS settable via `agent update` (`--mcp-config`), with `--mcp-config null` to
  clear; only `custom_env` is gated behind the dedicated env endpoint.
- "`agent get` shows env values." It shows only `has_custom_env` and
  `custom_env_key_count`.
- "An invalid `thinking_level`/`model` combo is caught at create." Only an
  unknown provider-level literal is — model-specific gaps fail at run time.
- "`set` and `add` are interchangeable for skills." `set` replaces all
  bindings; using it when you meant `add` silently removes capabilities.

## References

`references/creating-agents-source-map.md` maps every contract above to its
`file:line` on the current tree, the runtime effect, and a safe read-only
verification command.
$skill2$,
 '{"origin": {"type": "builtin"}}');

-- multica-creating-agents: references/creating-agents-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-creating-agents'),
 'references/creating-agents-source-map.md',
 $file2$# Creating agents — source map

Evidence layer for `SKILL.md`. Every contract maps to `file:line` on the
current tree (branch `feat/builtin-skills`, latest `main` merged), the runtime
effect, and a safe read-only check. Line numbers were re-derived against this
tree — re-derive again if the files move, the surrounding context (not the
number) is the anchor.

## Verification

```bash
# Conformance eval for this skill (and the shared template invariants):
go test ./internal/service -run TestCreatingAgentsSkillCoversAgentCreationContracts
go test ./internal/service -run TestBuiltinSkillsConformToTemplate
```

## CLI entry points — `server/cmd/multica/cmd_agent.go`

| Contract | Line | Behavior | Safe check |
|---|---|---|---|
| Create flags: `name`, `description`, `instructions`, `runtime-id` | 159–162 | Registered create flags; `name`/`runtime-id` enforced in `runAgentCreate` | `multica agent create --help` |
| `runtime-config`, `model`, `thinking-level`, `custom-args` flags | 163–166 | `model` help: "Prefer this over passing --model in --custom-args"; `thinking-level` is a thin pass-through (server validates the provider enum, empty = runtime default); `custom-args` help names codex/openclaw rejecting `--model` (CLI help only, not server-enforced) | `multica agent create --help` |
| Secret-safe env input: `custom-env`, `custom-env-stdin`, `custom-env-file` | 167–169 | `--custom-env` warns about shell history / `ps`; stdin and file modes keep secrets off the command line; mutually exclusive | `multica agent create --help` |
| Secret-safe MCP input: `mcp-config`, `mcp-config-stdin`, `mcp-config-file` (create) | 170–172 | Same three-channel pattern as `custom-env`; `--mcp-config` warns about shell history / `ps`; value must be a JSON object or `null` | `multica agent create --help` |
| MCP flags on `agent update` | 194–196 | Same three channels on update; `--mcp-config null` clears. Unlike `custom_env`, `mcp_config` IS settable via update | `multica agent update --help` |
| `thinking-level` flag on `agent update` | 184 | New reasoning/effort level; thin pass-through; `--thinking-level ""` clears to runtime default (mirrors `--model`) | `multica agent update --help` |
| `runAgentCreate` builds body + `POST /api/agents` | 419 | Only sets a body key when the flag `Changed`; posts to `/api/agents` (line 495) | read 419–496 |
| Body assembly: description/instructions/runtime-config/custom-args/custom-env/mcp-config/model/thinking-level | 438–488 | `resolveCustomEnv` (460) and `resolveMcpConfig` (465) gate their secret channels; `model` (470) and `thinking_level` (478) are `Changed`-gated pass-throughs; omitted flags are not sent | read 438–488 |
| `runAgentUpdate` sends `thinking_level` / `mcp_config` | 508 | `thinking_level` added when `--thinking-level` is `Changed` (556); `resolveMcpConfig` adds `mcp_config` (570); `PUT /api/agents/{id}` at 584; `custom_env` is intentionally not a flag here | read 508–585 |
| `parseMcpConfig` / `resolveMcpConfig` helpers | 1086, 1114 | Validator (object-or-`null`, content-free errors) + three-channel resolver, mirroring `parseCustomEnv`/`resolveCustomEnv` | read 1086–1170 |
| `agent skills set` = replace-all | 792 | `PUT /api/agents/{id}/skills` (810); `--skill-ids ''` clears all (798–799) | `multica agent skills set --help` |
| `agent skills add` = additive | 817 | `POST /api/agents/{id}/skills/add` (838); requires ≥1 id (823–828) | `multica agent skills add --help` |
| `agent skills list` | 760 | reads bindings, no side effect | `multica agent skills list --help` |
| `agent env get` | 894 | `GET /api/agents/{id}/env` | `multica agent env get --help` |
| `agent env set` | 929 | `PUT /api/agents/{id}/env` with full `custom_env` map (935, 949) | `multica agent env set --help` |

Note: the CLI no longer exposes `--from-template`. The agent-template backend
still exists (registry `server/internal/agenttmpl/`, handler `agent_template.go`,
routes `GET /api/agent-templates` and `POST /api/agents/from-template`, plus the
`packages/core` client/query wrappers) but is currently orphaned plumbing with no
live caller: the removed CLI flag was its only non-test consumer, and onboarding
does NOT use it — `packages/views/onboarding/steps/step-agent.tsx` builds four
hardcoded local presets (i18n-resolved) and creates via plain `POST /api/agents`
(`createAgent`), never `POST /api/agents/from-template`. Do not treat the template
API as a supported agent-creation path. This skill teaches manual `agent create`
only.

## Create handler — `server/internal/handler/agent.go`

| Contract | Line | Behavior |
|---|---|---|
| `maxAgentDescriptionLength = 255` | 31 | Cap is 255 **Unicode code points** (comment: counted via `utf8.RuneCountInString`, matches Postgres `char_length`) |
| `AgentResponse` omits plaintext `custom_env` | 33–53 | Exposes only `has_custom_env` (52) and `custom_env_key_count` (53); comment cites MUL-2600 |
| `CreateAgentRequest` fields | 565–585 | `description`, `instructions`, `runtime_config`, `custom_env`, `custom_args`, `model`, `thinking_level` (plus name/avatar/visibility/mcp_config/max_concurrent_tasks) |
| `name` required | 623–625 | 400 "name is required" |
| `description` ≤ 255 code points | 627–629 | `utf8.RuneCountInString(req.Description) > maxAgentDescriptionLength` → 400 |
| `runtime_id` required | 631–633 | `if req.RuntimeID == ""` → 400 "runtime_id is required" |
| `runtime_id` must resolve in workspace | 642–658 | parsed + `GetAgentRuntimeForWorkspace`; unknown → 400 "invalid runtime_id" |
| `thinking_level` provider-level validation | 673–676 | `!agent.IsKnownThinkingValue(runtime.Provider, req.ThinkingLevel)` → 400; per-model gaps deferred to daemon (comment 669–672, MUL-2339) |
| Defaults: `{}` config/env, `[]` args | 688–701 | `RuntimeConfig`→`{}`, `CustomEnv`→`{}`, `CustomArgs`→`[]` when nil, before insert |
| `visibility` default | 635–636 | `if req.Visibility == "" { req.Visibility = "private" }` — access-control field, not the runtime prompt |
| `max_concurrent_tasks` default | 638–639 | `if req.MaxConcurrentTasks == 0 { req.MaxConcurrentTasks = 6 }` — scheduler cap |
| `mcp_config` null-skip on create | 704–705 | raw JSON copied through unless the body value is the literal `null` |
| `mcp_config` redacted on read | 54, 848–851 | `redactMcpConfig` sets `McpConfigRedacted=true`; a private agent read by a member also redacts (494, 509) |
| `CreateAgent` insert params | 708–722 | persists runtime_config, instructions, custom_env, custom_args, model, thinking_level, mcp_config, visibility, max_concurrent_tasks |
| `UpdateAgent` rejects `custom_env` | 910–913 | if `custom_env` present in body → 400 "use PUT /api/agents/{id}/env (or `multica agent env set`)" |
| `UpdateAgent` persists / clears `mcp_config` | 944–948, 1060–1061 | Tri-state from the raw body: key omitted → no change; literal `null` → `ClearAgentMcpConfig`; object → replace. No 400 like `custom_env` — `mcp_config` IS updatable here |
| `description` ≤ 255 on update too | 921–924 | same cap re-checked on update |

## Env endpoint — `server/internal/handler/agent_env.go`

| Contract | Line | Behavior |
|---|---|---|
| `authorizeAgentEnv` gate | 66 | loads agent, then applies the two checks below |
| Agent actors denied | 80–84 | `if actorType == "agent"` → 403 "agents may not access env management endpoints" (MUL-2600 impersonation guard) |
| Owner/admin only | 86 | `requireWorkspaceRole(..., "owner", "admin")` |

## Routes — `server/cmd/server/router.go`

| Contract | Line | Behavior |
|---|---|---|
| `GET /env` | 603 | `h.GetAgentEnv` (plaintext read, gated) |
| `PUT /env` | 604 | `h.UpdateAgentEnv` (full-map overwrite, gated) |

## Claim-time injection — `server/internal/handler/daemon.go`

| Contract | Line | Behavior |
|---|---|---|
| Fresh agent re-read on claim | 1109–1111 | `GetAgent(task.AgentID)` — claim uses persisted fields, not create output |
| Workspace skills FIRST | 1115 | `skills := h.TaskService.LoadAgentSkills(...)` |
| Built-ins appended | 1116 | `skills = append(skills, h.TaskService.BuiltinSkills()...)` |
| Runtime payload | 1130–1143 | `TaskAgentData` carries `Instructions`, `Skills`, `CustomEnv`, `CustomArgs`, `Model`, `ThinkingLevel`, `McpConfig` (1130–1131, 1140) — confirms these are runtime-consumed; `description`, `visibility`, and `max_concurrent_tasks` are absent (not runtime-prompt fields) |

## Skill loading — `server/internal/service/task.go`

| Contract | Line | Behavior |
|---|---|---|
| `LoadAgentSkills` | 1685 | `ListAgentSkills` + per-skill `ListSkillFiles` → content + supporting files for execution |

## Built-in skills — `server/internal/service/builtin_skills.go`

| Contract | Line | Behavior |
|---|---|---|
| `go:embed builtin_skills` | 10–11 | skills embedded at compile time |
| `loadBuiltinSkill` | 45 | reads `<name>/SKILL.md` (47) + walks sibling files into `Files` (56–68) |

## Persisted columns — `server/pkg/db/generated/agent.sql.go`

| Contract | Line | Behavior |
|---|---|---|
| `CreateAgent` INSERT | 730–736 | columns include `runtime_config, runtime_id, instructions, custom_env, custom_args, mcp_config, model, thinking_level` |
| `CreateAgentParams` | 739–756 | typed params: `RuntimeConfig []byte`, `Instructions string`, `CustomEnv []byte`, `CustomArgs []byte`, `Model pgtype.Text`, `ThinkingLevel pgtype.Text` |
| `UpdateAgent` SET | 2552–2566 | COALESCE updates of `runtime_config, instructions, custom_env, custom_args, model, thinking_level` — note `custom_env` is COALESCE-guarded but the handler rejects it before this query runs |
| `UpdateAgentCustomEnv` (called by the `UpdateAgentEnv` handler) | 2652 | `SET custom_env = $2` — the only write path for env values |
$file2$);

-- multica-mentioning
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-mentioning',
 'Use when an issue comment needs to @mention someone — link to a person, trigger another agent, hand work to a squad, or broadcast with @all. Documents the verified mention contract: how a mention link is built from a real UUID, the four mention types and exactly what each one enqueues (agent → a run for that agent, squad → a run for the squad leader, member and issue → a rendered link with NO run), comment create/edit preview and suppression, the @all broadcast and how it suppresses the assignee's auto-trigger, and the silent no-op cases (a name where a UUID belongs, a bad/unknown UUID, an already-pending task, an archived agent, a private agent you cannot access). WHETHER to mention — loop avoidance, staying silent on acknowledgements — lives in the runtime brief's Mentions section, not here. This skill is the backend contract only, traced to server/internal/util/mention.go and server/internal/handler/comment.go.',
 $skill3$---
name: multica-mentioning
description: "Use when an issue comment needs to @mention someone — link to a person, trigger another agent, hand work to a squad, or broadcast with @all. Documents the verified mention contract: how a mention link is built from a real UUID, the four mention types and exactly what each one enqueues (agent → a run for that agent, squad → a run for the squad leader, member and issue → a rendered link with NO run), comment create/edit preview and suppression, the @all broadcast and how it suppresses the assignee's auto-trigger, and the silent no-op cases (a name where a UUID belongs, a bad/unknown UUID, an already-pending task, an archived agent, a private agent you cannot access). WHETHER to mention — loop avoidance, staying silent on acknowledgements — lives in the runtime brief's Mentions section, not here. This skill is the backend contract only, traced to server/internal/util/mention.go and server/internal/handler/comment.go."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Mentioning & Delegating

This skill states WHAT a mention link does in the Multica backend, traced to
source. WHETHER to mention at all — loop avoidance, staying silent on
acknowledgements — is in your runtime brief's Mentions section; follow that and
do not repeat it here.

Every claim below is pinned to source in
`references/mentioning-source-map.md`. If behavior ever differs from this
document, the source map is where to re-check it.

## A mention link is built from a real UUID

The backend recognizes a mention only through this Markdown shape:

    [@Label](mention://<type>/<id>)

The parser (`util.MentionRe` in `server/internal/util/mention.go`) accepts
exactly four `<type>` values plus the `all` sentinel, and the `<id>` group
accepts only hex characters and dashes, OR the literal string `all`:

    (member|agent|squad|issue|all)/([0-9a-fA-F-]+|all)

So the link target is a real entity UUID (or `all`), never a display name. The
label between the brackets is free text — that is where the human-readable name
goes.

## Step 1 — look up the UUID with `--output json`

A name is not a UUID. Look the UUID up first, from the matching list command:

- a person → `multica workspace member list --output json` → use `user_id`
- an agent → `multica agent list --output json` → use `id`
- a squad  → `multica squad list --output json` → use `id`

For a person the mention id is the `user_id`, NOT the membership-row id — the
backend's own roster formatter uses `user_id` for member mentions. Match by
display name. If the name is ambiguous or absent, do not guess — say so in your
comment instead of emitting a broken link.

## Step 2 — the four types and exactly what each enqueues

Format: `[@Name](mention://<type>/<uuid>)`. The `<type>` and the id source must
match, or the link resolves to the wrong entity (or to nothing).

| To…                  | type     | uuid from       | What the backend does                                    |
| -------------------- | -------- | --------------- | -------------------------------------------------------- |
| trigger an agent     | `agent`  | agent.id        | enqueues a run for that agent (`EnqueueTaskForMention`)  |
| hand work to a squad | `squad`  | squad.id        | resolves the squad's `leader_id` and enqueues a run for the LEADER agent |
| link a person        | `member` | member.user_id  | renders a link; enqueues NOTHING — no agent run          |
| reference an issue   | `issue`  | issue.id        | renders a link; enqueues NOTHING — always safe           |

The mention trigger set is computed by `computeMentionedAgentCommentTriggers`
(`server/internal/handler/comment.go`); the comment path folds that result into
`computeCommentAgentTriggers` and enqueues it via `enqueueCommentAgentTriggers`.
It acts on two types only: the `squad` branch resolves the squad and adds its
leader to the trigger set; everything that is not `agent` after that is skipped
(`if m.Type != "agent" { continue }`), then the `agent` branch adds that agent.
A `member` or `issue` mention reaches neither branch, so it enqueues no task.

A `member` mention therefore does NOT make a person "run", and this skill does
NOT claim it delivers a notification through the Go comment handler — there is
no such code path in that handler (see the source map). What is verified is the
contract above: only `agent` and `squad` mentions enqueue work.

## Preview and per-comment suppression

Newer clients can call `POST /api/issues/{id}/comments/trigger-preview` before
creating or editing a comment. The preview endpoint uses the same
`computeCommentAgentTriggers` function as create and edit re-triggering, so the
displayed agent chips come from backend rules, not from a client-side
reimplementation.

When previewing an edit, clients may send `editing_comment_id`. The server
validates that the comment belongs to the same workspace and issue, derives or
checks the edit's parent comment context, and excludes only pending tasks whose
`trigger_comment_id` is that same comment. Pending tasks from any other comment
on the issue still dedupe the preview.

When creating or editing a comment, clients may send an optional
`suppress_agent_ids` array. The server still computes the full trigger set
first, then removes those agent IDs as a post-filter. A missing or empty field
preserves the old behavior. A valid UUID that is not in the computed trigger set
is a no-op; a malformed UUID is rejected at the request boundary.

## @all is the broadcast type

`@all` uses the literal `all`, never a UUID:

    [@all](mention://all/all)

It addresses everyone on the issue. It does NOT make any specific agent run.
And it is special at trigger time: in `commentMentionsOthersButNotAssignee`
(`server/internal/handler/comment.go`), a comment that carries an `@all`
mention is treated as a broadcast that SUPPRESSES the issue assignee's
automatic on-comment trigger. Use `@all` to announce, not to request work from
the assignee.

## What does NOT happen (so the result doesn't surprise you)

These are all silent no-ops — no error, no run:

- **A name where a UUID belongs.** `mention://member/Alice` is dead. The id
  group accepts only hex+dashes or `all`; the non-hex letters in a typical name
  make the whole pattern fail to match, so the parser returns nothing.
- **A hex-ish but wrong UUID.** A well-formed-looking UUID that no entity owns
  DOES parse, then no-ops at lookup: the workspace-scoped query finds no agent
  and the loop `continue`s. Same agent-visible result (nothing fires), but the
  mechanism is the lookup miss, not a parse failure.
- **An already-pending task.** Even a correct `@agent`/`@squad` is skipped when
  the target already has a pending task on this issue
  (`HasPendingTaskForIssueAndAgent` → `continue`). Edit preview is the only
  exception: `editing_comment_id` ignores pending tasks from the same comment
  being edited, because save cancels those old tasks before it re-computes
  triggers. It is still comment-scoped, not an agent-wide bypass.
- **An archived agent**, or a squad whose leader is archived: skipped
  (`RuntimeID` invalid or `ArchivedAt` set).
- **A private agent you cannot access:** skipped — the mention path gates on
  `canAccessPrivateAgent` directly for both `@agent` and `@squad` (the
  `canEnqueueSquadLeader` wrapper is the assignment/child-done path, not this
  one).

## Incorrect → Correct

Incorrect: `@alice please review`
  → plain text, no link, parses to nothing, nobody is reached.

Incorrect: `[@Alice](mention://member/Alice) please review`
  → "Alice" is not a UUID; the id group rejects the non-hex letters, the
  pattern does not match, the link is silently dead.

Correct:
  1. `multica workspace member list --output json`  → Alice's `user_id` = 7f3a…
  2. `[@Alice](mention://member/7f3a…) please review`
     → a real `user_id` parses; the link renders and resolves to Alice.

@all broadcast: `[@all](mention://all/all) heads up` — addresses everyone,
runs no specific agent, and suppresses the assignee auto-trigger.

These exact shapes are pinned by a Go behavior test
(`TestMentioningSkillTeachesTheParserContract`) that feeds them through
`util.ParseMentions`: the name form parses to nothing, the real-UUID form
parses, `@all` parses to `{all, all}`, and a wrong `type` with a real UUID
still parses (which is why the type must match the id source).

## References

`references/mentioning-source-map.md` — file:line evidence for the regex, the
enqueue branches, the @all suppression, and the CLI id-source mapping, plus the
explicit note that no member-notification delivery path exists in the Go
comment handler.
$skill3$,
 '{"origin": {"type": "builtin"}}');

-- multica-mentioning: references/mentioning-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-mentioning'),
 'references/mentioning-source-map.md',
 $file3$# Mentioning — source map

Every claim in `SKILL.md` traces to a line below. Re-derive against the current
tree before trusting any line number; the behavior is the contract, the line is
a pointer.

## The mention grammar (what parses)

| Fact | Source |
| --- | --- |
| `MentionRe` — the only recognizer of a mention link | `server/internal/util/mention.go:16` |
| Pattern: `` `\[@?(.+?)\]\(mention://(member\|agent\|squad\|issue\|all)/([0-9a-fA-F-]+\|all)\)` `` | `server/internal/util/mention.go:16` |
| `<type>` group = `member \| agent \| squad \| issue \| all` | `server/internal/util/mention.go:16` |
| `<id>` group = `[0-9a-fA-F-]+` (hex + dashes) **or** the literal `all` — so a typical name with non-hex letters never matches | `server/internal/util/mention.go:16` |
| `ParseMentions` extracts and dedups `{Type, ID}` from `m[2]`/`m[3]` | `server/internal/util/mention.go:24-37` |
| `Mention.Type` doc enum = "member", "agent", "issue", or "all" (squad added in regex) | `server/internal/util/mention.go:7` |
| `HasMentionAll` reports whether any parsed mention is `all` | `server/internal/util/mention.go:40-47` |

### Parser behavior tests (pin the example shapes the skill uses)

| Case proven | Source |
| --- | --- |
| `mention://member/<real-uuid>` parses to `{member, uuid}` | `server/internal/util/mention_test.go:42-45` |
| `mention://all/all` parses to `{all, all}` | `server/internal/util/mention_test.go:47-50` |
| `mention://agent/<uuid>` parses; label may contain `[brackets]` | `server/internal/util/mention_test.go:13-35` |
| plain text with no `mention://` parses to `nil` | `server/internal/util/mention_test.go:57-60` |
| Skill eval: a name where a UUID belongs (`mention://member/Alice`) parses to `nil`; a bare `@name` parses to `nil`; a real UUID parses; `@all` → `{all, all}`; a **wrong** type with a real UUID still parses (points at the wrong entity) | `server/internal/service/builtin_skills_test.go:101-157` |

## What each mention type enqueues

| Fact | Source |
| --- | --- |
| `computeCommentAgentTriggers` is the shared comment trigger computation used by preview and enqueueing | `server/internal/handler/comment.go:1159-1195` |
| `computeMentionedAgentCommentTriggers` builds the mention trigger set; `enqueueCommentAgentTriggers` is the shared enqueue helper | `server/internal/handler/comment.go:1381-1467,1124-1157` |
| Comment creation runs `triggerTasksForComment`, which computes triggers, applies suppressions, then enqueues | `server/internal/handler/comment.go:1069,1092-1098` |
| Comment edit re-triggering also runs `triggerTasksForComment` after cancelling old tasks for the edited comment | `server/internal/handler/comment.go:1577-1594` |
| `squad` branch: resolve squad in workspace, read `LeaderID`, add the leader trigger | `server/internal/handler/comment.go:1397-1435` |
| `squad` → shared enqueue helper calls `EnqueueTaskForSquadLeader` | `server/internal/handler/comment.go:1141-1147` |
| Everything not `agent` after the squad branch is skipped: `if m.Type != "agent" { continue }` | `server/internal/handler/comment.go:1437-1439` |
| `agent` branch: load agent in workspace, then add the agent trigger | `server/internal/handler/comment.go:1440-1464` |
| `agent` → shared enqueue helper calls `EnqueueTaskForMention` (a run for that agent) | `server/internal/handler/comment.go:1148-1154` |
| **`member` and `issue` mentions reach neither branch — they enqueue NOTHING.** A `member` mention fails the `!= "agent"` skip at lines 1437-1439 (the squad branch above it only matches `squad`); an `issue` mention does the same. | `server/internal/handler/comment.go:1397,1437-1439` |

## Preview and suppression

| Fact | Source |
| --- | --- |
| Preview route: `POST /api/issues/{id}/comments/trigger-preview` | `server/cmd/server/router.go:707` |
| Preview handler loads the issue, expands issue identifiers, then calls `computeCommentAgentTriggers` | `server/internal/handler/comment.go:837-911` |
| Preview request accepts `content`, optional `parent_id`, and optional `editing_comment_id` | `server/internal/handler/comment.go:778-782` |
| Preview response returns agent `id`, `name`, optional `avatar_url`, `source`, and `reason` | `server/internal/handler/comment.go:784-793` |
| `editing_comment_id` is parsed as UUID input, scoped to the same workspace and issue, and used as `ExcludeTriggerCommentID` | `server/internal/handler/comment.go:855-872` |
| Preview validates or derives the parent context for an edit | `server/internal/handler/comment.go:874-897` |
| `CreateCommentRequest` accepts optional `suppress_agent_ids` | `server/internal/handler/comment.go:770-776` |
| `UpdateComment` accepts optional `suppress_agent_ids` | `server/internal/handler/comment.go:1509-1513` |
| Create-comment `suppress_agent_ids` is parsed as request-boundary UUID input | `server/internal/handler/comment.go:957-964` |
| Update-comment `suppress_agent_ids` is parsed as request-boundary UUID input | `server/internal/handler/comment.go:1523-1535` |
| Create and edit trigger paths compute the full trigger set, then apply `filterSuppressedCommentAgentTriggers` before enqueueing | `server/internal/handler/comment.go:1092-1122,1594` |
| Frontend API sends `editing_comment_id` for preview and `suppress_agent_ids` for update when present | `packages/core/api/client.ts:664-700` |
| Edit UI calls preview with `editingCommentId`, renders trigger chips, tracks suppressed agents, and submits suppressions on save | `packages/views/issues/components/comment-card.tsx:269-274,300-315,359-367,578-582,858-862` |
| Preview hook includes `editingCommentId` in its query key and sends it to the API | `packages/views/issues/hooks/use-comment-trigger-preview.ts:58-80` |
| Timeline edit mutation passes suppressed agent IDs through to the API layer | `packages/views/issues/hooks/use-issue-timeline.ts:299-302` |

## Edit-preview pending-task dedup

| Fact | Source |
| --- | --- |
| Default dedup query skips any queued or dispatched task for the issue and agent | `server/pkg/db/queries/agent.sql:544-548` |
| Edit-preview dedup query excludes only tasks whose `trigger_comment_id` equals the edited comment | `server/pkg/db/queries/agent.sql:550-558` |
| `hasPendingTaskForIssueAndAgent` selects the comment-scoped exclusion only when `ExcludeTriggerCommentID` is valid | `server/internal/handler/comment.go:1232-1244` |
| Agent-assignee on-comment dedup uses the shared helper | `server/internal/handler/issue.go:2576-2594` |
| Assigned squad leader on-comment dedup uses the shared helper | `server/internal/handler/comment.go:1197-1229` |
| Mentioned squad leader dedup uses the shared helper | `server/internal/handler/comment.go:1397-1435` |
| Direct agent mention dedup uses the shared helper | `server/internal/handler/comment.go:1440-1464` |
| Positive regression test covers all four edit-preview trigger sources | `server/internal/handler/comment_trigger_preview_test.go:179-265` |
| Negative regression test proves another comment's pending task still dedupes the preview | `server/internal/handler/comment_trigger_preview_test.go:267-290` |
| Edit-submit regression test proves `suppress_agent_ids` filters update-triggered tasks | `server/internal/handler/comment_trigger_preview_test.go:292-316` |

## Guards that make a valid mention a silent no-op

| Guard | Source |
| --- | --- |
| agent archived / no runtime → `continue` (`RuntimeID` invalid or `ArchivedAt` set) | `server/internal/handler/comment.go:1451-1452` |
| squad leader archived / no runtime → `continue` | `server/internal/handler/comment.go:1417-1423` |
| private agent the actor cannot access → `continue` (`canAccessPrivateAgent`) | `server/internal/handler/comment.go:1454-1458` |
| private squad leader the actor cannot trigger → `continue` (`canAccessPrivateAgent`) | `server/internal/handler/comment.go:1425-1428` |
| already-pending dedup (agent) → shared pending-task helper → `continue` | `server/internal/handler/comment.go:1459-1463` |
| already-pending dedup (squad leader) → shared pending-task helper → `continue` | `server/internal/handler/comment.go:1429-1433` |
| `canAccessPrivateAgent` definition | `server/internal/handler/agent_access.go` (search `func (h *Handler) canAccessPrivateAgent`) |
| `canEnqueueSquadLeader` (loads leader, delegates to `canAccessPrivateAgent`) | `server/internal/handler/agent_access.go:82-91` |

## @all broadcast and assignee-trigger suppression

| Fact | Source |
| --- | --- |
| `commentMentionsOthersButNotAssignee` — decides whether to suppress the assignee's on-comment trigger | `server/internal/handler/comment.go:1246-1288` |
| `@all` is treated as a broadcast → returns true → assignee auto-trigger suppressed | `server/internal/handler/comment.go:1257-1261` |
| Comment-flow computation that consults it | `server/internal/handler/comment.go:1175-1177` |
| `@all` never enqueues a specific agent: it is neither `squad` nor `agent`, so it is skipped in the mention trigger computation | `server/internal/handler/comment.go:1437-1439` |

## CLI id sources (where the UUID comes from)

| List command | Field used as mention id | Source |
| --- | --- | --- |
| `workspace member list` | `user_id` (NOT the membership-row id) | `server/cmd/multica/cmd_workspace.go:465` |
| `agent list` | `id` | `server/cmd/multica/cmd_agent.go:365` |
| `squad list` | `id` | `server/cmd/multica/cmd_squad.go:57` |
| Member mention uses `user_id`, confirmed by the backend roster formatter: `formatMention(user.Name, "member", userID)` where `userID = UUIDToString(m.MemberID)` | `server/internal/handler/squad_briefing.go:189-190` |
| `formatMention` emits `[@<name>](mention://<type>/<id>)` | `server/internal/handler/squad_briefing.go:216-218` |

## Explicit non-claim: no member-notification path in the Go comment handler

The skill deliberately does **not** assert that a `member` mention "sends a
notification." `server/internal/handler/comment.go` has no notification
delivery path for member (or issue) mentions: `computeMentionedAgentCommentTriggers`
branches only on `squad` and `agent`
(`server/internal/handler/comment.go:1397,1437-1439`), and a grep of the file for
`notif` returns only an unrelated comment about avoiding "log spam" on
unchanged threads — no member-notification call. The verified contract is
narrow: a `member` or `issue` mention renders as a link and enqueues no agent
run; only `agent` and `squad` mentions enqueue work. If a notification UX
exists, it is not in this handler, so this skill makes no claim about it.
$file3$);

-- multica-oss-operations
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-oss-operations',
 'Upload, download, and list files in the workspace Object Storage Service (OSS).',
 $skill4$---
name: multica-oss-operations
description: "Upload, download, and list files in the workspace Object Storage Service (OSS)."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica OSS Operations

You have access to a workspace-level Object Storage Service (OSS). Use it
to store files that need to persist beyond the current session — generated
reports, logs, screenshots, reference documents, and build artifacts.

## When to use OSS

- Save task outputs (reports, charts, logs, screenshots) for the user to review
- Store reference documents that agents in other tasks or sessions may need
- Share downloadable files with workspace members via public URLs

## Commands

```bash
# Upload a file — returns the public URL
multica oss upload --file <local_path> [--key <oss_key>]

# List uploaded files
multica oss list [--prefix <dir>]

# Download a file
multica oss download --key <oss_key> --output <local_path>
```

## Best practices

- Upload generated reports, logs, and screenshots at the end of each task
- Use descriptive keys with a directory structure:
  `reports/2026-06-24-sales-summary.pdf`, `logs/task-123-build.log`
- Reference OSS URLs in issue comments using markdown:
  `[Download report](https://files.example.com/reports/...)`
- Check `multica oss list` before overwriting an existing key
- Do NOT store credentials, secrets, or .env files in OSS
$skill4$,
 '{"origin": {"type": "builtin"}}');

-- multica-projects-and-resources
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-projects-and-resources',
 'Use when creating, inspecting, updating, or debugging Multica projects and project resources. Covers durable project context, github_repo and local_directory resources, how resources affect future agent task context, when to bind repos, and when not to mutate resources.',
 $skill5$---
name: multica-projects-and-resources
description: "Use when creating, inspecting, updating, or debugging Multica projects and project resources. Covers durable project context, github_repo and local_directory resources, how resources affect future agent task context, when to bind repos, and when not to mutate resources."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica Projects and Resources

## Quick start

Projects are durable context containers. Resources attached to a project can affect future agent tasks.

```bash
multica project list --output json
multica project get <project-id> --output json
multica project resource list <project-id> --output json
```

Project resources are mutated through project resource commands/endpoints. Issue
comments do not create durable project resources.

## Core model

A project groups work and carries durable resources. A resource is not just display metadata; it is context later injected into task briefs and `.multica/project/resources.json`.

A project's `description` is also durable context: when an issue (or a quick-create task) is bound to a project, the project description is injected into the agent's brief under `## Project Context` and written to `.multica/project/resources.json` as `project_description`. Use it for project-wide rules/context that should apply to every task in the project.

Common resource types:

- `github_repo` — durable GitHub repo context, with `resource_ref.url`, optional checkout `ref`, and optional prompt-only `default_branch_hint`;
- `local_directory` — daemon-local path context, with `resource_ref.local_path`, `daemon_id`, and optional label.

## CLI

```bash
multica project list --output json
multica project get <project-id> --output json
multica project create --title "<title>" --repo <github-url> --output json
multica project update <project-id> --title "<title>" --output json
multica project status <project-id> in_progress --output json
multica project resource list <project-id> --output json
multica project resource add <project-id> --type github_repo --url <github-url> --output json
multica project resource add <project-id> --type github_repo --url <github-url> --ref <branch-or-sha> --output json
multica project resource add <project-id> --type local_directory --local-path <abs-path> --daemon-id <daemon-id> --output json
multica project resource update <project-id> <resource-id> --url <new-github-url> --output json
multica project resource update <project-id> <resource-id> --ref <branch-or-sha> --output json
multica project resource remove <project-id> <resource-id> --output json
```

For `github_repo`, non-JSON `--ref` sets `resource_ref.ref`, the default checkout branch/tag/SHA for future tasks in that project. JSON `--ref '<json>'` remains the escape hatch for full payloads or resource types not covered by shortcuts.

## When to add a resource

Add/update a project resource when the user asks for durable project context: "把这个 GitHub repo 绑到项目上", "以后都用这个 repo", "agent 总是拿不到这个项目的仓库", or "这个项目要在我的本地目录里跑".

Project resources are durable and affect future tasks. `multica repo checkout`
is task-local checkout state.

## Debugging wrong context

1. `multica project get <project-id> --output json`.
2. `multica project resource list <project-id> --output json`.
3. Check `github_repo.resource_ref.url`, optional `ref`, `default_branch_hint`, and `local_directory.resource_ref.daemon_id`.
4. Updating resources is a durable mutation. After an update, listing the
   resource is the verification path.
5. If resources match the expected task context, inspect runtime/repo checkout
   path next.

## Side effects

Project create/update/delete/status and project resource add/update/remove mutate durable workspace state and affect future tasks. Ask before changing `local_directory` unless the user explicitly requested that exact local path.

More source-backed details: `references/projects-and-resources-source-map.md`.
$skill5$,
 '{"origin": {"type": "builtin"}}');

-- multica-projects-and-resources: references/projects-and-resources-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-projects-and-resources'),
 'references/projects-and-resources-source-map.md',
 $file5$# Projects and resources source map

- `server/cmd/multica/cmd_project.go` registers project `list`, `get`, `create`, `update`, `delete`, and `status`.
- The same file registers `project resource list/add/update/remove`.
- `project create --repo` attaches `github_repo` resources during project creation.
- `project resource add` supports shortcuts for `github_repo` (`--url`, non-JSON `--ref` for checkout ref, `--default-branch-hint`) and `local_directory` (`--local-path`, `--daemon-id`, `--ref-label`), or generic JSON `--ref '<json>'`.
- `project resource update` merges shortcut edits with existing `resource_ref` so a partial edit does not clobber required fields; non-JSON `--ref` updates `github_repo.resource_ref.ref`.
- `server/cmd/server/router.go` exposes `/api/projects` plus `/api/projects/{projectId}/resources` routes.
- `server/pkg/db/queries/project_resource.sql` is the CRUD query surface for `project_resource` rows.
- Project resources are written into `.multica/project/resources.json` for agent workdirs.
- `github_repo.resource_ref.ref` is lifted into daemon `RepoData.Ref` by `server/internal/handler/daemon.go`; `server/internal/daemon/daemon.go` stores it per task, and `server/internal/daemon/health.go` uses it as the default `/repo/checkout` ref when the checkout request does not explicitly pass one.
- A project's `description` is injected as durable context for every task in the project. The claim handler (`server/internal/handler/daemon.go`) reads `proj.Description` onto the claim response (`ProjectDescription`, `server/internal/handler/agent.go`); the daemon carries it through `Task` (`server/internal/daemon/types.go`) and `TaskContextForEnv` (`server/internal/daemon/execenv/execenv.go`) into the brief's `## Project Context` section (`server/internal/daemon/execenv/runtime_config.go`) and into `.multica/project/resources.json` as `project_description` (`server/internal/daemon/execenv/context.go`).
$file5$);

-- multica-runtimes-and-repos
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-runtimes-and-repos',
 'Use when inspecting or debugging Multica runtimes, daemon task claiming, agent not running, workdir/session reuse, or repository checkout. Covers runtime online/offline state, daemon heartbeat/claim chain, task-scoped repo checkout, project repo context, local_directory caveats, and safe diagnostic commands.',
 $skill6$---
name: multica-runtimes-and-repos
description: "Use when inspecting or debugging Multica runtimes, daemon task claiming, agent not running, workdir/session reuse, or repository checkout. Covers runtime online/offline state, daemon heartbeat/claim chain, task-scoped repo checkout, project repo context, local_directory caveats, and safe diagnostic commands."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica Runtimes and Repos

## Quick start

For "agent did not run" or "repo checkout failed", read the chain before changing anything:

```bash
multica agent get <agent-id> --output json
multica runtime list --output json
multica repo checkout <repo-url>
```

Runtime and repo commands affect active agent execution. Do not restart daemons, update runtimes, or check out arbitrary repos just to test.

## Core model

A runtime is the execution target behind an agent. A daemon owns local runtime processes and claims queued tasks from the server.

The chain is:

1. user action creates or updates an `agent_task_queue` row;
2. the task points at an agent and runtime;
3. server wakes the runtime over daemon websocket when possible;
4. daemon polls/claims the task;
5. server returns task context, repos, project resources, prior session/workdir hints, and task token;
6. daemon prepares a workdir and launches the provider CLI;
7. `multica repo checkout` talks to the local daemon, not directly to GitHub.

## CLI

```bash
multica runtime list --output json
multica runtime usage <runtime-id> --output json
multica runtime activity <runtime-id> --output json
multica runtime update <runtime-id> --target-version <version> --output json
multica runtime delete <runtime-id>
multica repo checkout <url>
multica repo checkout <url> --ref <branch-or-sha>
```

`runtime update` and `runtime delete` are writes. `runtime delete` removes a runtime registration; if active agents are still bound, it refuses unless the user explicitly passes `--cascade`, which archives those agents and cancels their queued/running tasks before deleting the runtime. `repo checkout` creates a git worktree in the task working directory.

`repo checkout` requires `MULTICA_DAEMON_PORT`; it is intended to run inside a daemon task. If absent, you are not in the normal agent checkout path. When a project `github_repo` resource has `resource_ref.ref`, `repo checkout <url>` uses that ref by default for the current task; an explicit `repo checkout <url> --ref <branch-or-sha>` overrides it.

## Debugging an agent that did not run

Check in this order:

1. Was a task supposed to be created? Inspect issue/comment/autopilot context.
2. Is the assignee an agent or squad? A squad routes to its leader.
3. Is the agent archived or bound to a runtime the actor cannot use?
4. Is the runtime online? `multica runtime list --output json`.
5. Did the daemon heartbeat recently? Runtime `last_seen_at` is the visible clue.
6. Did the task get claimed or is it stuck pending/running/waiting for local directory?
7. If repo checkout failed, classify it after checking whether repo context was
   present in the task/project context.

## Repos

The runtime brief lists repos available to this task. Treat that list as the authority for agent checkout unless the user explicitly asks to bind a new project resource.

Workspace repos and project resources are not the same thing:

- workspace repo metadata can appear in workspace context;
- `github_repo` project resources are durable project context and can affect future tasks; optional `resource_ref.ref` pins the default checkout ref for tasks in that project;
- `local_directory` resources point at a path owned by a daemon and carry local-machine assumptions.

Do not add a project resource just because `repo checkout` failed. First determine whether the user asked for durable project context or just a task checkout.

More source-backed details: `references/runtimes-and-repos-source-map.md`.
$skill6$,
 '{"origin": {"type": "builtin"}}');

-- multica-runtimes-and-repos: references/runtimes-and-repos-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-runtimes-and-repos'),
 'references/runtimes-and-repos-source-map.md',
 $file6$# Runtimes and repos source map

- `server/cmd/multica/cmd_runtime.go` registers `runtime list`, `usage`, `activity`, `update`, and `delete`.
- `runtime list` reads `/api/runtimes` and prints `id`, `name`, `runtime_mode`, `provider`, `status`, and `last_seen_at`.
- `runtime update` posts to `/api/runtimes/{runtime-id}/update`; with `--wait` it polls update status.
- `runtime delete` deletes `/api/runtimes/{runtime-id}`; with `--cascade`, it first reads the `runtime_has_active_agents` conflict payload and posts those ids to `/api/runtimes/{runtime-id}/archive-agents-and-delete`.
- `server/cmd/multica/cmd_repo.go` registers `repo checkout <url> [--ref]`.
- `repo checkout` requires `MULTICA_DAEMON_PORT`, sends `workspace_id`, `workdir`, `ref`, `agent_name`, and `task_id` to local daemon `/repo/checkout`, then prints the checked-out path.
- `server/internal/daemon/health.go` resolves the checkout ref: request `ref` wins; otherwise it asks `server/internal/daemon/daemon.go` for the current task's project repo default ref.
- `server/cmd/server/router.go` registers daemon APIs under `/api/daemon`, including workspace repos and task claim.
- `server/internal/daemon/daemon.go` claims tasks, prepares workdirs, launches provider CLIs, and reports completion.
- `server/internal/daemon/execenv/runtime_config.go` injects task/project/repo context into agent workdirs.
$file6$);

-- multica-skill-importing
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-skill-importing',
 'Use when a user provides a skill URL, slug, or clear intent to import/install a specific skill into the current Multica workspace. Teaches the workspace import API/CLI path (POST /api/skills/import), the supported URL source families, --on-conflict fail|overwrite|rename|skip behavior and structured import results, additive agent binding vs replace-all, and the reserved SKILL.md supporting-file rule. Do not use it to decide which skill the user needs, and never treat an external local installer like npx skills add as the final Multica install.',
 $skill7$---
name: multica-skill-importing
description: "Use when a user provides a skill URL, slug, or clear intent to import/install a specific skill into the current Multica workspace. Teaches the workspace import API/CLI path (POST /api/skills/import), the supported URL source families, --on-conflict fail|overwrite|rename|skip behavior and structured import results, additive agent binding vs replace-all, and the reserved SKILL.md supporting-file rule. Do not use it to decide which skill the user needs, and never treat an external local installer like npx skills add as the final Multica install."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Importing skills into Multica

Use this skill when the user already provided a skill URL, slug, or a clear intent
to import a specific skill into the current Multica workspace.

Do not use this skill to decide which skill the user needs. If the user only
describes a capability and no URL is known, external search may produce candidate
URLs, but this import skill starts only once a URL or concrete import target is
known.

Every claim below is traced to source in
`references/skill-importing-source-map.md`. When in doubt, read that file.

## The invariant

A skill is installed for Multica only when it exists in the current workspace's
skill database. The single supported path that puts it there is the workspace
import endpoint, driven by this CLI:

```bash
multica skill import --url <url> --output json
```

The CLI defaults to `--on-conflict fail`. Current CLIs send:

```text
POST /api/skills/import
body: { "url": "<url>", "on_conflict": "fail" }
```

Do not finish with `npx skills add`. That installs into an external/local skill
environment, not the Multica workspace DB, so Multica cannot manage or bind it.

## Supported URL source families

`detectImportSource` accepts these hosts (and `www.` variants). Pass any of these
forms to `multica skill import --url <url> --output json`:

```bash
multica skill import --url clawhub.ai/owner/skill --output json
multica skill import --url skills.sh/owner/repo/skill --output json
multica skill import --url github.com/owner/repo --output json
multica skill import --url github.com/owner/repo/tree/main/path/to/skill --output json
multica skill import --url github.com/owner/repo/blob/main/path/to/SKILL.md --output json
```

- `clawhub.ai`, `skills.sh`, `github.com` are the recognized hosts.
- A GitHub URL may be a bare `owner/repo`, a `/tree/{ref}/...` directory, or a
  `/blob/{ref}/.../SKILL.md` file.
- A bare ClawHub slug (no host) is accepted and routed to ClawHub.
- Any other host is rejected with a 400 naming the supported sources.

## Direct URL flow

1. When the request contains a concrete URL, the import endpoint can be called
directly; search is not required by the API:

```bash
multica skill import --url <url> --output json
```

2. Treat the response as the source of truth. Current CLI imports use the
structured import result envelope:

```json
{
  "status": "created|updated|conflict|skipped|failed",
  "reason": "...",
  "skill": { "...": "SkillWithFilesResponse when created/updated" },
  "existing_skill": { "id": "...", "name": "...", "can_overwrite": true }
}
```

For `created` / `updated`, `skill` is a workspace `SkillWithFilesResponse`: it
embeds the standard `SkillResponse` and adds the supporting `files` array. Report
the relevant fields:

- `status` and `reason` when present.
- `skill.id` / `skill.name` / `skill.description`.
- `skill.config.origin` (provenance: which source the skill was imported from —
  set only when the source supplied an origin, so treat it as possibly absent).
- `skill.files` / files count.
- `skill.created_at` / `skill.updated_at`.
- `existing_skill.id` / `existing_skill.name` when status is `conflict`,
  `skipped`, or `failed` due to an existing skill.

Because the response is structured, read these returned fields instead of guessing
whether the import succeeded.

3. Agent-skill binding is a separate mutable operation. `add` preserves existing
assignments and appends the new id:

```bash
multica agent skills add <agent-id> --skill-ids <skill-id> --output json
multica agent skills list <agent-id> --output json
```

After the final `multica agent skills list <agent-id> --output json`, verify the
target skill id is present before claiming the skill is available to that agent.

## Additive add vs replace-all set

`multica agent skills add` is additive: the server inserts the assignments without
clearing existing ones (`AddAgentSkills`).

`multica agent skills set` is replace-all: the server clears every current
assignment, then re-adds exactly the ids you pass (`SetAgentSkills`).
`set` is the replacement path. Passing only one id to `set` leaves the agent with
only that one skill and drops every previous assignment.

## Reserved SKILL.md supporting file

A skill's primary content is its `SKILL.md`. That filename is reserved: the daemon
writes the primary content to `SKILL.md` itself when preparing the execution
environment, so a *supporting* file may not also be named `SKILL.md`
(`IsReservedContentPath`; the check cleans the path and is case-insensitive, so
`./SKILL.md` and `sub/../SKILL.md` are caught too).

Practical effect when importing or creating a skill: if the manifest lists a
supporting file named `SKILL.md`, the server silently drops it — the import still
succeeds, but that entry will be absent from the returned `files`. So if a
supporting file you expected is missing, check whether it was named `SKILL.md`;
rename it to a non-reserved path. (The hard `400` rejection — "SKILL.md is reserved
for the primary skill content" — only fires on the dedicated single-file endpoint
`PUT /api/skills/{id}/files`, not on import.)

## Same-name conflicts: `--on-conflict`

Default behavior is safe: `multica skill import --url <url>` is equivalent to
`--on-conflict fail`. If the imported skill name already exists, the command
prints a structured `conflict` result and exits non-zero; no skill is created or
updated.

Choose an explicit strategy only when the user asked for it or the intent is
clear:

- `--on-conflict fail` (default): do nothing on conflict; report `status:
  conflict` with a reason that suggests overwrite or rename.
- `--on-conflict overwrite`: update the existing same-name skill in place, but
  only if the current user is the skill's original creator. This preserves the
  skill ID, `created_by`, `created_at`, and agent-skill bindings; it replaces
  description, content, provenance config, and supporting files. Non-creators get
  `status: failed`.
- `--on-conflict rename`: create a new skill with an automatic suffix such as
  `-2` / `-3`; the existing skill is untouched.
- `--on-conflict skip`: leave the existing skill untouched and report `status:
  skipped`.

Concrete examples:

```bash
# Safe default. Fails with status=conflict if review-helper already exists.
multica skill import --url https://skills.sh/acme/repo/review-helper --output json

# Replace the existing same-name skill, preserving its ID and agent bindings.
multica skill import --url https://skills.sh/acme/repo/review-helper --on-conflict overwrite --output json

# Keep the existing skill and import a copy such as review-helper-2.
multica skill import --url https://skills.sh/acme/repo/review-helper --on-conflict rename --output json

# Batch-friendly behavior: leave the existing skill alone and mark it skipped.
multica skill import --url https://skills.sh/acme/repo/review-helper --on-conflict skip --output json
```

Legacy compatibility: clients that do not send `on_conflict` keep the old
contract. A duplicate import returns `409` and the body carries the existing
workspace skill identity:

```json
{
  "error": "a skill with this name already exists",
  "existing_skill": {
    "id": "<skill-id>",
    "name": "<skill-name>"
  }
}
```

Current CLI normalizes that legacy shape into `status: conflict` and exits
non-zero for the default `fail` strategy. Treat `existing_skill.id` and
`existing_skill.name` as the source of truth, then fetch details if needed:

```bash
multica skill get <skill-id> --output json
```

Older servers may return a `409` whose body is only a string like `a skill with
this name already exists`, with no `existing_skill` key. Recover by finding the
existing workspace skill yourself:

```bash
multica skill list --output json
multica skill get <skill-id> --output json
```

Then report that the skill already exists and include its `id` / `name`. Do not
retry in a loop, and do not create a second skill under a different name just to
dodge the conflict.

## Incorrect → correct

Incorrect (bypasses Multica):

```bash
npx skills add https://skills.sh/owner/repo/skill
```

The skill may exist locally, but Multica cannot manage it as a workspace skill.

Incorrect agent binding for a normal add (replaces every existing assignment):

Using `set` with only the new skill id wipes the agent's other skills. For an add,
use `add`.

Correct import:

```bash
multica skill import --url https://skills.sh/owner/repo/skill --output json
```

Agent binding after import, when the caller intentionally wants to mutate that
agent's skill assignments:

```bash
multica agent skills add <agent-id> --skill-ids <skill-id> --output json
multica agent skills list <agent-id> --output json
```

## References

- `references/skill-importing-source-map.md` — every behavior above mapped to
  `file:line` in `server/`, plus the verification command to re-derive the lines.
$skill7$,
 '{"origin": {"type": "builtin"}}');

-- multica-skill-importing: references/skill-importing-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-skill-importing'),
 'references/skill-importing-source-map.md',
 $file7$# Skill-importing source map

Evidence layer for `multica-skill-importing`. Every behavioral claim in `SKILL.md`
maps to a real code path below with `file:line`. Paths are relative to the repo
root (`multica/`).

Re-derive before trusting: line numbers drift. To re-verify a single anchor,
`grep` the symbol and read its surroundings, e.g.:

```bash
grep -n "func (h \*Handler) ImportSkill" server/internal/handler/skill.go
grep -n "func runSkillImport"           server/cmd/multica/cmd_skill.go
grep -n "func IsReservedContentPath"    server/internal/skill/reserved.go
```

## Import endpoint and route

| Behavior | File:line |
|---|---|
| `ImportSkill` handler (`POST /api/skills/import`) | `server/internal/handler/skill.go:1882` |
| Decodes `ImportSkillRequest` (`{ "url": ..., "on_conflict": ... }`) | `server/internal/handler/skill.go:1895-1899`, struct at `:553` |
| Validates `on_conflict` (`fail`, `overwrite`, `rename`, `skip`) | `server/internal/handler/skill.go:1900-1908`, helper `validImportOnConflict` at `:566` |
| Detects source family + normalizes URL | `server/internal/handler/skill.go:1910` (calls `detectImportSource`) |
| Persists provenance into `config.origin` | `server/internal/handler/skill.go:1944-1948` — set only when `imported.origin != nil`; otherwise `config` stays `{}` and `origin` is absent |
| Structured conflict dispatcher | `server/internal/handler/skill.go:1813-1878` |
| Builds skill + files via `createSkillWithFiles` (def `server/internal/handler/skill_create.go:77`, tx body `:29`) | wrapped by `createImportedSkillWithName` at `server/internal/handler/skill.go:1774` |
| Structured success: `201 Created` with `{status:"created", skill}` when `on_conflict` was sent | `server/internal/handler/skill.go:1985-1988` |
| Legacy success: `201 Created` with bare `SkillWithFilesResponse` when `on_conflict` was omitted | `server/internal/handler/skill.go:1990` |
| Route registration `r.Post("/import", h.ImportSkill)` | `server/cmd/server/router.go:874` |

## CLI: `multica skill import --url`

| Behavior | File:line |
|---|---|
| `skill import` command def | `server/cmd/multica/cmd_skill.go:60-64` |
| `--url` flag | `server/cmd/multica/cmd_skill.go:142` |
| `--on-conflict` flag (default `fail`) | `server/cmd/multica/cmd_skill.go:143` |
| `--output` flag (default `json`) | `server/cmd/multica/cmd_skill.go:144` |
| `runSkillImport` | `server/cmd/multica/cmd_skill.go:412` |
| Requires `--url` | `server/cmd/multica/cmd_skill.go:418-421` |
| Reads and validates `--on-conflict` | `server/cmd/multica/cmd_skill.go:422-425` |
| Sends `on_conflict` in the request body | `server/cmd/multica/cmd_skill.go:428-431` |
| `POST /api/skills/import` | `server/cmd/multica/cmd_skill.go:436` |
| Structured HTTP error body handling | `server/cmd/multica/cmd_skill.go:437-440`, `handleSkillImportError` at `:454` |
| Prints structured result (`json` or table) | `server/cmd/multica/cmd_skill.go:443`, helper at `:497` |

## Same-name conflict handling

| Behavior | File:line |
|---|---|
| `SkillImportResult` (`status`, `reason`, `skill`, `existing_skill`) | `server/internal/handler/skill.go:104-109` |
| `ExistingSkillIdentity` (`id`, `name`, `created_by`, `can_overwrite`) | `server/internal/handler/skill.go:112-117` |
| Pre-create lookup for structured conflict flow | `server/internal/handler/skill.go:1951-1962` |
| Race-safe unique-violation fallback into structured conflict flow | `server/internal/handler/skill.go:1966-1971` |
| Default `fail`: `status:"conflict"` and HTTP 409 | `server/internal/handler/skill.go:1872-1877` |
| `overwrite`: creator-only update, preserves skill identity/bindings via `overwriteSkillWithFiles` | `server/internal/handler/skill.go:1823-1852`, tx helper at `server/internal/handler/skill_create.go:133` |
| `rename`: creates suffixed name with bounded attempts | `server/internal/handler/skill.go:1854-1870`, helper at `:1786` |
| `skip`: returns `status:"skipped"` and leaves existing skill untouched | `server/internal/handler/skill.go:1816-1821` |
| Legacy duplicate branch when `on_conflict` was omitted | `server/internal/handler/skill.go:1973-1978` |
| Legacy duplicate response `{error, existing_skill}` | `server/internal/handler/skill.go:118-123` |
| CLI normalizes legacy `{existing_skill}` body into `status:"conflict"` | `server/cmd/multica/cmd_skill.go:454-482`, helper at `:484` |

## Response shape: `SkillWithFilesResponse`

| Behavior | File:line |
|---|---|
| `SkillWithFilesResponse` = embedded `SkillResponse` + `Files []SkillFileResponse` | `server/internal/handler/skill.go:99-102` |
| `SkillResponse` fields (`id, workspace_id, name, description, content, config, created_by, created_at, updated_at`) | `server/internal/handler/skill.go:41-51` |
| `SkillFileResponse` fields | `server/internal/handler/skill.go:80-87` |
| `createSkillWithFilesInTx` returns `SkillWithFilesResponse{SkillResponse, Files}` | `server/internal/handler/skill_create.go:66-69` |
| `config.origin` set on import | `server/internal/handler/skill.go:1947` |

For current CLI imports, `SkillWithFilesResponse` appears under
`SkillImportResult.skill` when status is `created` or `updated`. Legacy clients
that omit `on_conflict` still receive a bare `SkillWithFilesResponse`.

## URL source families (`detectImportSource`)

| Behavior | File:line |
|---|---|
| `detectImportSource` | `server/internal/handler/skill.go:773-804` |
| `skills.sh` / `www.skills.sh` | `server/internal/handler/skill.go:791-792` |
| `clawhub.ai` / `www.clawhub.ai` | `server/internal/handler/skill.go:793-794` |
| `github.com` / `www.github.com` | `server/internal/handler/skill.go:795-796` |
| Bare slug (no host) defaults to ClawHub | `server/internal/handler/skill.go:798-800` |
| `parseGitHubURL` handles `/tree/{ref}/...` and `/blob/{ref}/.../SKILL.md` | `server/internal/handler/skill.go:1450-1503` (tree/blob check `:1463-1480`) |

## Additive add vs replace-all set

| Behavior | File:line |
|---|---|
| `AddAgentSkills` (additive: AddAgentSkill loop, no RemoveAll) | `server/internal/handler/skill.go:2161`; loop `:2192-2200` |
| Route `POST /api/agents/{id}/skills/add` | `server/cmd/server/router.go:851` |
| `SetAgentSkills` (replace-all: RemoveAllAgentSkills then re-add) | `server/internal/handler/skill.go:2106`; `RemoveAllAgentSkills` `:2138`; re-add `:2143-2151` |
| Route `PUT /api/agents/{id}/skills` | `server/cmd/server/router.go:850` |
| CLI `agent skills add` def ("without replacing existing assignments") | `server/cmd/multica/cmd_agent.go:125-130` |
| `runAgentSkillsAdd` → `POST .../skills/add` | `server/cmd/multica/cmd_agent.go:797`; POST `:818` |
| CLI `agent skills set` def ("replaces all current assignments") | `server/cmd/multica/cmd_agent.go:118-123` |
| `runAgentSkillsSet` → `PUT .../skills` | `server/cmd/multica/cmd_agent.go:772`; PUT `:790` |
| CLI `agent skills list` | `server/cmd/multica/cmd_agent.go:740`; GET `:750` |

## Reserved primary-content filename (`SKILL.md`)

| Behavior | File:line |
|---|---|
| `ContentFilename = "SKILL.md"` | `server/internal/skill/reserved.go:12` |
| `IsReservedContentPath` (cleans path, case-insensitive compare) | `server/internal/skill/reserved.go:25-27` |
| Import/create path: reserved supporting file is **silently skipped** (`continue`) | `server/internal/handler/skill_create.go:50-54` |
| `UpdateSkill` (PUT `/api/skills/{id}`) replace-files path: also silently skips | `server/internal/handler/skill.go:490-494` |
| `UpsertSkillFile` (PUT `/api/skills/{id}/files`): **rejects 400** "SKILL.md is reserved for the primary skill content" | `server/internal/handler/skill.go:2014`; reserved check `:2034-2036` |

Reason `SKILL.md` is reserved: the daemon writes the skill's `Content` to that path
itself when preparing the execution environment, so a supporting file may not also
claim it (`server/internal/skill/reserved.go:8-24`).

Behavior is path-shape-dependent. On **import or create** a manifest's `SKILL.md`
supporting file is dropped (it will not appear in the returned `files`), so the
import still succeeds — it does not 400. The hard 400 rejection fires only on the
dedicated single-file endpoint `PUT /api/skills/{id}/files`.
$file7$);

-- multica-squads
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-squads',
 'Use when creating, inspecting, updating, assigning, mentioning, or debugging Multica squads. Explains what squads are, squad/member fields, CLI commands, leader routing, issue assignment, comments, mentions, autopilot behavior, leader briefing, side effects, and product-gap handling.',
 $skill8$---
name: multica-squads
description: "Use when creating, inspecting, updating, assigning, mentioning, or debugging Multica squads. Explains what squads are, squad/member fields, CLI commands, leader routing, issue assignment, comments, mentions, autopilot behavior, leader briefing, side effects, and product-gap handling."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica Squads

## Quick start

If debugging why a squad did or did not run, inspect first:

```bash
multica issue get <issue-id> --output json
multica squad get <squad-id> --output json
multica squad member list <squad-id> --output json
multica issue comment list <issue-id> --recent 10 --output json
```

If the command shape is unclear, check help instead of guessing:

```bash
multica squad --help
multica squad member --help
multica issue update --help
multica issue comment add --help
```

Do not assign, comment, mention, update, delete, or record squad activity just
to test. These can mutate workspace state or trigger agent runs.

## Core model

A Multica squad is a workspace routing and coordination object.

A squad is not an agent. It does not run work by itself. Current behavior:
squad-routed work runs through the squad's `leader_id` agent.

Important consequences:

- assigning an issue to a squad routes to the leader;
- mentioning a squad routes to the leader;
- squad-assigned autopilot resolves to the leader;
- squad members are not automatically fanned out;
- squad `instructions` are leader briefing content, not member prompts.

## CLI

Squad commands:

```bash
multica squad list --output json
multica squad get <squad-id> --output json
multica squad create --name <name> --leader <agent-name-or-id> --output json
multica squad update <squad-id> --instructions "<leader coordination policy>" --output json
multica squad delete <squad-id>
```

Member commands:

```bash
multica squad member list <squad-id> --output json
multica squad member add <squad-id> --member-id <id> --type agent|member --role <role> --output json
multica squad member remove <squad-id> --member-id <id> --type agent|member
multica squad member set-role <squad-id> --member-id <id> --member-type agent|member --role <role> --output json
```

Squad leader evaluation command:

```bash
multica squad activity <issue-id> action|no_action|failed --reason "<why>" --output json
```

`activity` is a write: it records the leader's evaluation decision on an issue.
Use it only when acting as the squad leader after evaluating a trigger.

Issue/comment commands often needed with squads:

```bash
multica issue get <issue-id> --output json
multica issue update <issue-id> --help
multica issue comment list <issue-id> --output json
multica issue comment add <issue-id> --help
```

Prefer `--output json` for reads. Use `--help` before writes.

## Squad fields

- `id` — squad UUID.
- `workspace_id` — workspace the squad belongs to.
- `name` — display name; unique per workspace.
- `description` — human-facing metadata/display text. Do not assume runtime
  prompt impact unless source proves a consumer.
- `instructions` — squad-level instructions added to the squad leader briefing.
  They are not directly injected into every squad member.
- `avatar_url` — optional squad avatar URL.
- `leader_id` — agent ID of the squad leader; the runtime target for
  squad-routed work.
- `creator_id` — creator of the squad.
- `archived_at` / `archived_by` — archive metadata. Archived squads are rejected
  by assignment/autopilot routing paths.
- `member_count` — list response count of squad members.
- `member_preview` — list response preview of squad members.

Use `instructions` for leader-facing coordination policy: squad responsibility,
delegation expectations, when to ask humans, and review/handoff rules. Do not
write it as if every member automatically receives it.

## Squad member fields

- `member_type` — `agent` or `member`.
- `member_id` — ID of the agent or workspace member.
- `role` — roster role label. Current behavior: non-empty `role` appears in the
  leader briefing roster. Do not assume it creates scheduling, permissions, or
  routing behavior.

## Creation and leader membership

Creating a squad requires `leader_id`. The leader must be a workspace agent.
Create/update does not reject an archived leader: the lookup only checks the
agent exists in the workspace. An archived leader fails closed later, at
routing/dispatch — assignment, autopilot admission, and the comment/mention
readiness gate all reject an archived leader before any task is enqueued.

On create, the backend attempts to add the leader as a squad member with role
`leader`. When updating `leader_id`, if the new leader is not already a member,
the backend adds the new leader as a squad member with role `leader`.

## Leader briefing

For squad leader tasks, Multica appends a squad leader briefing to the leader
agent instructions. The briefing includes:

- Squad Operating Protocol;
- Squad Roster;
- Squad Instructions, only when `instructions` is non-empty.

Roster entries include member name, member type, mention markdown, and non-empty
role. For agent members the roster also lists their assigned skills
(`skills: a, b`, or `no skills assigned` when the agent has none) so the leader
can delegate by capability instead of guessing from the role label; human
members carry no skills segment. Builtin `multica-*` skills are not listed —
only the workspace skills explicitly attached to the agent. Archived agent
members are skipped from the briefing roster.

## Issue assignment behavior

Issues can be assigned to squads with:

```text
assignee_type = "squad"
assignee_id = <squad-id>
```

Current behavior:

- assignment routes work to `squad.leader_id`;
- it does not enqueue every squad member;
- assignment while status is `backlog` does not immediately start work;
- moving a squad-assigned issue out of `backlog` can trigger the leader;
- changing assignee cancels existing tasks for the issue before enqueueing the
  new assignee path.

Assignment validation rejects a missing type/id pair, non-existent squad,
archived squad, archived leader, and private leader when the actor cannot access
it.

## Comment and mention behavior

If an issue is assigned to a squad, a new comment can wake the squad leader. This
is leader routing, not member fan-out.

Squad mention format:

```md
[@Squad Name](mention://squad/<squad-id>)
```

Current behavior: resolve the squad, read `leader_id`, enqueue a leader task,
and use the current comment as the trigger comment. It does not enqueue every
squad member.

## Autopilot behavior

Autopilots can be assigned to squads. For `assignee_type = "squad"`:

- executable agent resolves from `squad.leader_id`;
- admission/readiness checks run against the leader;
- archived squads fail closed / skip dispatch;
- run attribution records squad id where applicable.

For `create_issue` autopilots, the created issue keeps `assignee_type = "squad"`
and `assignee_id = <squad-id>`, while the actual executing agent is the resolved
leader. For `run_only` autopilots, no issue is created; the task is created
directly for the resolved leader agent.

## Handling complaints or product gaps

When the user says squad behavior is wrong, confusing, or disappointing, do not
immediately assume code is broken and do not defend current behavior just because
it exists. Classify first:

- expected current behavior;
- configuration issue;
- product limitation;
- actual bug.

Explain the current source-backed behavior. If the behavior is technically
correct but product-wise bad, say so and propose a scoped product/code change.

Do not silently change squad routing, member fan-out, leader briefing, autopilot
behavior, or comment-trigger behavior without confirmation. These are product
contract changes with side effects.

## Side effects

These actions can trigger agent work or mutate durable state:

- creating a squad;
- updating squad fields;
- changing `leader_id`;
- adding/removing members;
- changing member roles;
- assigning an issue to a squad;
- moving a squad-assigned issue out of backlog;
- commenting on a squad-assigned issue;
- mentioning a squad;
- creating or triggering squad-assigned autopilots;
- recording squad activity with `multica squad activity`;
- deleting/archive squad.

Do not perform side-effecting actions as tests unless the user explicitly
authorizes them.

## Common wrong assumptions

- A squad is not an agent.
- Squad work routes to `leader_id`, not every member.
- Squad mention routes to the leader, not every member.
- Squad assignment routes to the leader, not every member.
- Squad autopilot resolves to the leader as executable agent.
- `instructions` are leader briefing content, not automatic member prompts.
- `description` is not proven runtime prompt content.
- `role` is roster context, not automatic scheduling.
- Backlog assignment does not immediately start work.

## References

For source paths, tests, edge cases, and exact routing details, see:

```text
references/squad-source-map.md
```
$skill8$,
 '{"origin": {"type": "builtin"}}');

-- multica-squads: references/squad-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-squads'),
 'references/squad-source-map.md',
 $file8$# Squad Source Map

This file records source evidence for `multica-squads/SKILL.md`.

Use this when the task requires exact source paths, edge-case behavior, tests, or contract verification.

## Object Model

### DB shape

Source:

```text
server/migrations/084_squad.up.sql                # base table: name, description, leader_id, creator_id
server/migrations/085_squad_archive.up.sql        # archived_at, archived_by columns
server/migrations/088_squad_instructions.up.sql   # instructions column
server/pkg/db/queries/squad.sql
packages/core/types/squad.ts
```

Key facts:

- `squad` stores `name`, `description`, `leader_id`, `creator_id` (084), archive
  metadata `archived_at`/`archived_by` (085), and `instructions` (088).
- `squad_member` stores `member_type`, `member_id`, and `role`.
- `member_type` is constrained to `agent` or `member`.
- issue `assignee_type` supports `squad`.

## CLI

Source:

```text
server/cmd/multica/cmd_squad.go
```

Commands:

```bash
multica squad list
multica squad get <squad-id>
multica squad create
multica squad update <squad-id>
multica squad delete <squad-id>
multica squad activity <issue-id> <outcome>

multica squad member list <squad-id>
multica squad member add <squad-id>
multica squad member remove <squad-id>
multica squad member set-role <squad-id>
```

Use `--help` for exact flags before writes.

## Create / Update

Source:

```text
server/internal/handler/squad.go                  # CreateSquad ~200-272, UpdateSquad ~287-364
server/pkg/db/queries/agent.sql                   # GetAgentInWorkspace ~15-17
server/pkg/db/generated/agent.sql.go              # getAgentInWorkspace ~1261
```

Contracts:

- create requires `leader_id` (squad.go:215-218);
- leader must be a workspace agent — both create (squad.go:230-237) and update
  (squad.go:333-338) validate via `GetAgentInWorkspace`;
- archived leader is NOT rejected at create/update: `GetAgentInWorkspace` is
  `WHERE id = $1 AND workspace_id = $2` (agent.sql:15-17) with no archived
  filter, so an archived agent can be set as leader here. Archived-leader fails
  closed later, at routing/dispatch — see the readiness gate (squad.go:945,
  isSquadLeaderReady → service.AgentReadiness at squad.go:1017), assignment
  validation (issue.go:2625-2627), and autopilot admission (autopilot.go:885-891);
- leader is auto-added as member with role `leader` (squad.go:258-263);
- updating `leader_id` auto-adds new leader as member if missing (squad.go:340-347).

## Leader Briefing

Source:

```text
server/internal/handler/squad_briefing.go         # buildSquadLeaderBriefing ~104, buildSquadRoster ~121, renderMemberRow ~169, agentSkillsRosterSegment, formatRosterRow
server/internal/handler/daemon.go                  # briefing injection ~1187, ~1530
```

Contracts:

- squad leader tasks append briefing to leader agent instructions
  (daemon.go:1187, 1530);
- briefing includes operating protocol, roster, and optional instructions
  (squad_briefing.go:104-117);
- `instructions` section appears only when non-empty (squad_briefing.go:110-112);
- archived agent members are skipped from roster (squad_briefing.go:178-179);
- agent member roster rows list assigned workspace skills via
  `loadSquadMemberSkillNames` (ListAgentSkillNamesByAgentIDs) and
  `agentSkillsRosterSegment` — "skills: a, b" or
  "no skills assigned"; builtin multica-* skills are excluded and human
  members carry no skills segment (squad_briefing.go renderMemberRow);
- no traced behavior injects `instructions` into every squad member.

## Issue Assignment

Source:

```text
server/internal/handler/issue.go                  # assignee validation ~2614-2632
server/internal/handler/squad.go                   # shouldEnqueueSquadLeaderOnAssign ~990, enqueueSquadLeaderTask ~1027
server/internal/service/task.go
```

Contracts:

- `assignee_type="squad"` routes to `squad.leader_id` (squad.go:1028-1050);
- backlog assignment does not immediately enqueue (squad.go:991-993);
- moving out of backlog can enqueue leader (squad.go:990-994 → isSquadLeaderReady);
- assignee change cancels existing issue tasks first;
- private leader access is checked at assign-time (issue.go:2629-2632) and at
  enqueue-time via `canEnqueueSquadLeader` (squad.go:1037);
- archived squad / archived leader rejected at assign-time (issue.go:2622-2627);
- pending task dedup is applied (squad.go:1042-1048).

## Comment / Mention

Source:

```text
server/internal/handler/comment.go                # comment triggers ~1057-1199, squad mention branch ~1352
server/internal/handler/squad.go                   # enqueueSquadLeaderTask ~986 (assign/backlog paths), lastTaskWasLeader ~915
server/internal/service/task.go                   # EnqueueTaskForSquadLeader
```

Contracts:

- commenting on a squad-assigned issue can wake the leader — the comment path
  computes triggers via `computeCommentAgentTriggers` (comment.go:1124), whose
  assigned-squad branch is `computeAssignedSquadLeaderCommentTrigger`
  (comment.go:1162-1199); the same computation backs the trigger-preview
  endpoint;
- explicit `mention://squad/<id>` resolves squad and adds the leader trigger
  (comment.go:1352-1391);
- squad mention does not fan out to members — enqueue targets `squad.LeaderID`
  only (comment.go:1104-1112, and squad.go:1007 on the assign/backlog paths);
- leader task uses `is_leader_task=true` (via `EnqueueTaskForSquadLeader`);
- leader self-trigger loops are guarded — same-leader / last-task-was-leader
  guards (comment.go:1173-1176, lastTaskWasLeader at squad.go:915) and member
  explicit-mention skip (comment.go:1177-1179).

## Autopilot

Source:

```text
server/internal/service/autopilot.go              # resolveAutopilotLeader ~617-655, dispatch ~88-111
server/internal/handler/autopilot.go              # save-time validateAutopilotAssignee ~845-893
```

Contracts:

- squad autopilot resolves executable agent from `squad.leader_id` —
  `resolveAutopilotLeader` squad branch (autopilot.go:639-651);
- readiness/admission checks target the leader: save-time validation rejects an
  archived squad/leader (handler/autopilot.go:881-891), and dispatch re-runs
  `resolveAutopilotLeader` + `AgentReadiness`;
- archived squad fails closed / skips dispatch — `errSquadArchived`
  (autopilot.go:644-645);
- `create_issue` keeps the issue assigned to the squad (autopilot.go:88-97);
- `run_only` creates task directly for leader (autopilot.go:99-106, dispatch via
  `resolveAutopilotLeader` at autopilot.go:284).

## Child-done Parent Trigger

Source:

```text
server/internal/handler/issue_child_done.go       # dispatchParentAssigneeTrigger ~246, triggerChildDoneSquad ~304
```

Contracts:

- when child issue completes and parent is assigned to squad, parent squad
  leader can be triggered (triggerChildDoneSquad at issue_child_done.go:304);
- routing is leader-only — one `EnqueueTaskForSquadLeader` on the leader, no
  member fan-out (issue_child_done.go:214-216, 344);
- loop guards skip same squad, same effective leader, and shared-leader
  cross-squad cases (issue_child_done.go:229-235, effectiveChildAgentOwner ~367,
  childAssigneeIsSquad ~387).

## Private Leader Access

Source:

```text
server/internal/handler/agent_access.go           # canAccessPrivateAgent ~25-40, canEnqueueSquadLeader ~82-91
server/internal/handler/squad.go                   # enqueueSquadLeaderTask gate ~1037
```

Contracts:

- public leaders pass — `canAccessPrivateAgent` returns true when
  `agent.Visibility != "private"` (agent_access.go:26-28);
- agent-to-agent traffic is allowed — `actorType == "agent"` short-circuits
  (agent_access.go:29-31);
- private leader access for members is limited to owner/admin or agent owner
  (agent_access.go:32-39);
- system triggers are treated like agent triggers for squad leader enqueue:
  `canEnqueueSquadLeader` remaps `actorType == "system"` to `"agent"` before
  delegating to `canAccessPrivateAgent` (agent_access.go:87-90). This is wired
  into `enqueueSquadLeaderTask`, which denies the enqueue when the actor cannot
  access the leader (squad.go:1037).

## Tests

Relevant test groups:

```text
server/internal/handler/squad_assign_trigger_test.go
server/internal/handler/squad_comment_trigger_test.go
server/internal/handler/squad_briefing_test.go
server/internal/handler/squad_private_leader_test.go
server/internal/handler/autopilot_private_leader_test.go
server/internal/handler/squad_no_action_test.go
```

Verification command:

```bash
go test ./internal/handler -run 'Test.*Squad|Test.*squad|Test.*Autopilot.*Squad|Test.*ChildDone.*Squad'
```
$file8$);

-- multica-wiki-currate
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-currate',
 'Use when ingesting raw notes from raw/learnings/ and other raw/ sources into the polished wiki/. Deduplicates and merges task-agent notes by topic into wiki/<topic>.md and wiki/pitfalls/<topic>.md so the knowledge base stays curated, not fragmented.',
 $skill9$---
name: multica-wiki-currate
description: "Use when ingesting raw notes from raw/learnings/ and other raw/ sources into the polished wiki/. Deduplicates and merges task-agent notes by topic into wiki/<topic>.md and wiki/pitfalls/<topic>.md so the knowledge base stays curated, not fragmented."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Curation

You are the wiki curator. Task agents dump raw knowledge notes into
`raw/learnings/<topic>-<task-id>.md`. Your job is to read those notes,
deduplicate and merge them by topic, and write the polished, interlinked
knowledge into `wiki/<topic>.md` and `wiki/pitfalls/<topic>.md`.

This is the curator tier of the two-tier knowledge pipeline: raw notes
(written fast by busy task agents) → curated wiki (written by you for
reuse).

## 1. List the raw notes awaiting curation

```bash
multica wiki list-pages --search "raw/learnings/"
```

Also scan `raw/` for any uncaptured sources.

## 2. Group by topic

Read each raw note (`multica wiki read-page --path <path>`) and group by
the topic slug (the `<topic>` segment of the path, or the note's stated
domain). Notes on the same topic merge into one `wiki/<topic>.md`.

## 3. Read the existing wiki page (if any)

```bash
multica wiki read-page --path "wiki/<topic>.md"
multica wiki read-page --path "wiki/pitfalls/<topic>.md"
```

Understand what's already curated so you merge, not duplicate.

## 4. Deduplicate and write the curated page

Merge the raw notes' conclusions + pitfalls into the existing page (or
create it). Deduplicate repeated insights across notes. Structure:

```markdown
# <Topic>

## Knowledge Summary
<merged paragraphs: the durable insights across all notes>

## Key Takeaways
<bullet list: one-line actionable insights>

## Pitfalls
<merged, deduplicated bullet list: mistakes + root causes + avoidance>

## Sources
- [[raw/learnings/<topic>-<task-id>]] — <one-line note summary>
```

```bash
multica wiki write-page --path "wiki/<topic>.md" --content "<merged content>"
```

If the topic has distinct pitfalls worth their own page, also write
`wiki/pitfalls/<topic>.md` with the merged pitfall list.

## 5. Update the index

Append a line to `wiki/index.md` (create if absent):

```bash
multica wiki write-page --path "wiki/index.md" --append "- [[<topic>]] — <one-line topic summary>"
```

## 6. Stop

Curated notes are now in `wiki/`. Leave the `raw/learnings/` notes in
place as the audit trail (or archive per workspace policy). The next
curation run picks up any newly-added notes.
$skill9$,
 '{"origin": {"type": "builtin"}}');

-- multica-wiki-distill
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-distill',
 'Use after completing a task that involved debugging, a workaround, a surprising failure, or a reusable pattern. Writes a raw note under raw/learnings/ for the wiki admin to curate, and optionally appends a pointer to an existing wiki/ page for high-value knowledge.',
 $skill10$---
name: multica-wiki-distill
description: "Use after completing a task that involved debugging, a workaround, a surprising failure, or a reusable pattern. Writes a raw note under raw/learnings/ for the wiki admin to curate, and optionally appends a pointer to an existing wiki/ page for high-value knowledge."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Knowledge Distillation

When you complete a task that produced reusable knowledge — a debugging
breakthrough, a workaround, an undocumented constraint, a root cause you
traced — distill it into the workspace wiki so the next agent (and you on
the next turn) doesn't rediscover or repeat the same mistakes.

This skill writes RAW notes to `raw/learnings/`. A wiki admin agent
curates those into the polished `wiki/` pages asynchronously. Only touch
`wiki/` directly to append a pointer to an EXISTING page when the insight
is urgent (step 4).

## When to invoke

Invoke when you:
- Solved a problem after multiple failed attempts.
- Discovered a constraint, limitation, or undocumented behavior.
- Used a workaround or non-obvious technique.
- Traced a production error to its root cause.
- Changed a system configuration that affects other agents.

Do NOT invoke for trivial tasks, simple CRUD, or tasks where the solution
was straightforward and already well-documented.

## 1. Pick a topic slug

A short, stable, lowercase-hyphenated slug for the domain:
`auth`, `db`, `deployment`, `agent-config`, `api-design`, etc.
If the knowledge spans topics, pick the most important and cross-link
the others inside the note body.

## 2. Write the raw note

Use a unique `<task-id>` (the issue id or a short slug) so notes never
collide:

```bash
multica wiki write-page --path "raw/learnings/<topic>-<task-id>.md" --content "$(cat <<'NOTE'
# <one-line summary>

## Background
<what the task was, what went wrong or what you needed>

## Conclusion
<the actionable insight another agent needs — one paragraph>

## Steps to Reproduce / Avoid
<concrete commands, config snippets, or patterns; or "n/a">

## Pitfalls
<bullet list: what went wrong, why, how to avoid it>

## Context
- Task: <issue identifier or one-line description>
- Date: <today>
- Agent: <your name>
NOTE
)"
```

## 3. (Optional) Point an existing wiki page at the note

Only if the knowledge is high-value AND a relevant `wiki/<topic>.md` or
`wiki/pitfalls/<topic>.md` already exists:

```bash
multica wiki read-page --path "wiki/<topic>.md"
multica wiki write-page --path "wiki/<topic>.md" --append "- <one-line takeaway> — see [[raw/learnings/<topic>-<task-id>]]"
```

Do NOT create new `wiki/` pages here — curation is the wiki admin's job.
You only add a pointer to an EXISTING page when the insight is urgent.

## 4. Stop

You're done. The wiki admin agent curates `raw/learnings/*` into the
polished `wiki/` pages on its schedule. Do not wait for or trigger that.
$skill10$,
 '{"origin": {"type": "builtin"}}');

-- multica-wiki-index-refresh
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-index-refresh',
 'Use when asked to 'refresh the index', 'rebuild index', or after batch ingests — to rebuild wiki/index.md from the current page catalog.',
 $skill11$---
name: multica-wiki-index-refresh
description: "Use when asked to 'refresh the index', 'rebuild index', or after batch ingests — to rebuild wiki/index.md from the current page catalog."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Index Refresh

Rebuild `wiki/index.md` — the primary navigation aid. Every page must be listed.

## Workflow

1. **List every page:** `multica wiki list-pages`
2. **Group by category:**

   | Prefix | Category |
   |--------|----------|
   | `wiki/sources/` | Sources |
   | `wiki/projects/` | Projects |
   | `wiki/areas/` | Areas |
   | `wiki/entities/` | Entities |
   | `wiki/concepts/` | Concepts |
   | `wiki/synthesis/` | Synthesis |

3. **For each page**, write a one-line summary: `- [[path]] — summary`
4. **Write the new index:**
   ```bash
   multica wiki write-page --path wiki/index.md --content "..."
   ```
   The index follows:
   ```markdown
   # Wiki Index
   ## Sources | ## Projects | ## Areas | ## Entities | ## Concepts | ## Synthesis
   ```
5. **Append to `wiki/log.md`:**
   ```
   ## [YYYY-MM-DD] index-refresh | <N> pages indexed
   ```

## Validation
- [ ] Every page from `multica wiki list-pages` appears in the index
- [ ] No stale entries (pages listed but deleted)
- [ ] Summaries factual and one line each
- [ ] Categories match the domain-driven layout
$skill11$,
 '{"origin": {"type": "builtin"}}');

-- multica-wiki-ingest
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-ingest',
 'Use when asked to ingest a captured source from raw/ into the wiki, or when the user says 'ingest <slug>'. Turns a source document into durable, interlinked wiki knowledge.',
 $skill12$---
name: multica-wiki-ingest
description: "Use when asked to ingest a captured source from raw/ into the wiki, or when the user says 'ingest <slug>'. Turns a source document into durable, interlinked wiki knowledge."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Ingest

Turn one source document into durable, interlinked wiki knowledge. Every page compounds.

## Domain-driven layout

```
wiki/
  index.md        # catalog — update on every ingest
  log.md          # append-only timeline
  sources/        # summary pages per ingested source
  projects/       # project overviews, standups, decisions
  areas/          # domain areas (PARA-style knowledge mapping)
  entities/       # people, orgs, products, places, technologies
  concepts/       # ideas, frameworks, definitions, patterns
  synthesis/      # cross-cutting analysis, comparisons, theses
```

## Workflow

1. **Read context.**
   ```bash
   multica wiki read-page --path wiki/index.md
   multica wiki read-page --path wiki/log.md | tail -20
   ```
2. **Read the source** with `multica wiki read-source --id <id>`.
3. **Plan** 3–5 takeaways. Confirm with user if they're in the loop.
4. **Write `wiki/sources/<slug>.md`** — ~300–800 words, frontmatter, neutral voice.
5. **Update/create downstream pages** in `areas/`, `entities/`, `concepts/`, `synthesis/`.
   Typical ingest touches 5–15 pages.
6. **Wire cross-links.** Every claim cites its source via `(see [[wiki/sources/slug]])`.
7. **Flag contradictions** — append `> ⚠ contradicted by [[...]] (YYYY-MM-DD)` to older page.
8. **Refresh `wiki/index.md`** — add one-line summaries for new pages.
9. **Append to `wiki/log.md`:** `## [YYYY-MM-DD] ingest | <title>`
$skill12$,
 '{"origin": {"type": "builtin"}}');

-- multica-wiki-lint
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-lint',
 'Use when asked to audit the wiki — 'lint', 'health check', 'audit'. Read-only. Return a triage list grouped by severity. Do not auto-fix.',
 $skill13$---
name: multica-wiki-lint
description: "Use when asked to audit the wiki — 'lint', 'health check', 'audit'. Read-only. Return a triage list grouped by severity. Do not auto-fix."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Lint

Audit, do not edit. Return findings the maintainer can triage.

## Workflow

1. **Walk the wiki:** `multica wiki read-page --path wiki/index.md` + `multica wiki list-pages`
2. **Check the seven recurring issues:**

   1. **Contradictions** — incompatible claims on two pages. Quote evidence.
   2. **Stale claims** — superseded by a newer source.
   3. **Orphan pages** — no inbound links from index or other pages.
   4. **Concept gaps** — term on 3+ pages with no dedicated concept page.
   5. **Broken `[[wiki-links]]`** — target does not exist.
   6. **Weak provenance** — uncited claims or circular wiki-only citations.
   7. **Index/log drift** — pages missing from index, or vice versa. Log entries
      without corresponding page changes.

3. **Return a triage list by severity:**
   - **Critical**: contradictions, broken links to active pages
   - **Medium**: stale claims, weak provenance, large concept gaps
   - **Low**: orphans, log drift, small index gaps

   Each finding: file path + evidence + suggested fix + follow-up operation.

4. **Do not write to `wiki/`** except `wiki/log.md`:
   ```
   ## [YYYY-MM-DD] lint | <N findings, M critical>
   - critical: <count>
   - medium: <count>
   - low: <count>
   ```

## Domain-specific checks
- `areas/` — all domain areas listed in `index.md`?
- `concepts/` — frequently-referenced term missing a concept page?
- `entities/` — entity referenced by name but lacking a page?
- `sources/` — raw source without a `wiki/sources/` page?
- `synthesis/` — substantial query answer not yet filed?

## Voice
Lead with severity counts. One finding per bullet. When unsure, flag medium with "verify".
$skill13$,
 '{"origin": {"type": "builtin"}}');

-- multica-wiki-maintain
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-maintain',
 'Use when maintaining the wiki — resolving contradictions, merging duplicates, restructuring directories, curating stale content, or evolving the schema. Maintenance work, not fresh ingest.',
 $skill14$---
name: multica-wiki-maintain
description: "Use when maintaining the wiki — resolving contradictions, merging duplicates, restructuring directories, curating stale content, or evolving the schema. Maintenance work, not fresh ingest."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Maintain

Curate, correct, restructure, and evolve the wiki's knowledge quality.

## Responsibilities

**Resolve contradictions** — When `> ⚠ contradicted by [[...]]` appears: read both claims,
determine truth, update page, remove callout, log resolution.

**Merge duplicates** — Two pages covering the same topic: merge unique content into the
stronger page, replace weaker with a redirect (`# Redirect\n\nSee [[wiki/...]]`), update
backlinks, refresh `wiki/index.md`.

**Restructure** — Rename/move pages: update all `[[wiki-links]]`, write at new path, delete
old, update index and log.

**Curate stale content** — Review pages with no updates in 90+ days. Flag outdated pages
with `stale_since` frontmatter. Offer to archive obsolete ones.

**Evolve schema** — When the domain outgrows categories: propose to user, update
`AGENTS.md`, create directory markers, update `wiki/index.md`, log the change.

Voice: terse, factual, neutral. You are the curator, not the author of new claims.
$skill14$,
 '{"origin": {"type": "builtin"}}');

-- multica-wiki-query
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-query',
 'Use when asked a question that the wiki might answer — search, read, synthesize, and file findings. Check the wiki before guessing or using external knowledge.',
 $skill15$---
name: multica-wiki-query
description: "Use when asked a question that the wiki might answer — search, read, synthesize, and file findings. Check the wiki before guessing or using external knowledge."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Query

Answer questions from wiki content. File substantial answers so the work compounds.

## Workflow

1. **Read `wiki/index.md`** to find candidate pages.
2. **Search** for additional candidates:
   ```bash
   multica wiki search --query "<keywords>"
   ```
3. **Batch-read** the most relevant pages. Follow links to connected pages.
4. **Synthesize** an answer with inline citations: `(see [[wiki/sources/foo]])`.
5. **Offer to file** the answer. If substantial:
   ```bash
   multica wiki write-page --path wiki/synthesis/<slug>.md --content "..."
   ```

## Search priority by category

1. `concepts/` — definitions and frameworks
2. `areas/` — domain-specific knowledge
3. `entities/` — specific people, orgs, technologies
4. `sources/` — original source summaries
5. `synthesis/` — prior cross-cutting analysis

## If the wiki lacks information

Say so plainly: "The wiki doesn't cover X. Suggested sources to ingest: ..."
Never fabricate citations or claim coverage that doesn't exist.

## Voice
Answer with citations. Prefer wiki content over external knowledge. File substantial
answers so the next query starts from a higher baseline.
$skill15$,
 '{"origin": {"type": "builtin"}}');

-- multica-working-on-issues
INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-working-on-issues',
 'Use when working on a Multica issue after the runtime has provided the trigger context — to apply the product contracts the runtime brief does not encode: how PR linking differs from close intent, how to read a linked PR's real state via the pull-requests CLI, which metadata keys are high-signal, what status changes trigger on the server, and how sub-issue create status (todo vs backlog) controls whether assigned agents start immediately.',
 $skill16$---
name: multica-working-on-issues
description: "Use when working on a Multica issue after the runtime has provided the trigger context — to apply the product contracts the runtime brief does not encode: how PR linking differs from close intent, how to read a linked PR's real state via the pull-requests CLI, which metadata keys are high-signal, what status changes trigger on the server, and how sub-issue create status (todo vs backlog) controls whether assigned agents start immediately."
user-invocable: false
allowed-tools: Bash(multica *), Bash(git *), Bash(gh *)
---

# Working on Multica issues

Product contracts the runtime brief does not fully encode: PR linking vs close
intent, reading linked-PR state, metadata keys, status side effects, and
sub-issue enqueue behavior.

For building mention links, load `multica-mentioning` instead — not this skill.

Every contract below is traced to source in
`references/working-on-issues-source-map.md`.

## PR linking and close intent are two distinct contracts

The GitHub webhook runs two separate scans over an incoming PR. They are not the
same gate and they read different fields.

**Linking** scans the PR **title, body, OR branch** for a routable issue key
(`PREFIX-NUMBER`, e.g. `MUL-2759`). Each match writes an issue ↔ PR link row.
This is the link that `multica issue pull-requests` reads back.

```text
MUL-2759: add built-in issue working skill        # title prefix → links
agent/matt/mul-2759-working-on-issues             # branch ref   → links
```

**Close intent** is stricter and is a separate scan over **title or body only —
never the branch**. It fires only for a key placed immediately after a closing
keyword (`Closes` / `Fixes` / `Resolves`, optional `:` then whitespace). That
adjacency is what sets the link row's close-intent flag, the gate that
auto-advances the issue to `done` when the PR merges.

```text
Closes MUL-2759                                    # links AND records close intent
Fixes MUL-2759
Resolves MUL-2759
Fix login MUL-2759                                 # links only — keyword not adjacent
```

Consequence: a bare title prefix or a branch reference links the PR but does not
close the issue on merge. A closing keyword immediately adjacent to the issue key
records close intent; on merge, that close intent can move the linked issue to
`done`.

### Default for code-changing issue work

When an issue run changes code in a checked-out GitHub repo, the default handoff
is to open or update a PR before posting the final Multica issue comment, unless
the user explicitly asked for a local-only change or no PR. This is a default, not
an unconditional command: if no code changed, say no PR is needed; if PR creation
is blocked by auth, failing tests, or missing remote state, report that blocker
instead of pretending the run is complete.

Use a routable issue key in the PR title, body, or branch so the webhook can link
the PR back to the issue. If the PR should close the issue on merge, put the key
immediately after a closing keyword in the title or body, for example:

```text
MUL-2759: fix login redirect        # links only
Closes MUL-2759                     # links and records close intent
```

In the final issue comment, include the PR URL when a PR exists. If the task did
not produce a PR because no code changed or the user asked not to create one, say
that explicitly.

## Reading a linked PR's real state

When a step depends on PR state, query Multica's link table — do not infer it
from branch names, GitHub search, memory, or `pr_url` metadata (which can be
stale).

```bash
multica issue pull-requests <issue-id> --output json
```

Returns `{"pull_requests": [...]}`. Each element exposes:

- `number`, `html_url`, `title`
- `state` — the PR lifecycle as a **single enum**, one of `merged`, `closed`,
  `draft`, `open`. There is no separate `draft` or `merged` boolean in the
  response; the server folds them into `state` (merged wins, then closed, then
  draft, else open).
- `merged_at` — non-null once merged; a second confirmation of `state: merged`.
- `mergeable_state` — mirrors GitHub (`clean` / `dirty` surfaced; other values
  round-trip as unknown).
- `checks_conclusion` — aggregated CI: `passed`, `failed`, `pending`, or `null`
  when no check suite has been observed. Backed by `checks_passed`,
  `checks_failed`, `checks_pending` counts.

So "is it merged?" is `state == "merged"` (or `merged_at != null`); "is it still
a draft?" is `state == "draft"`; CI status is `checks_conclusion`.

If the command returns no linked PRs after a PR was opened, the link scanner did
not observe a routable issue key in the PR title/body/branch.

## Metadata: high-signal keys only

Metadata is durable issue state. Reading metadata is safe. Writing a metadata key
is a state mutation and should be tied to an explicit task requirement to record
that state for later readers or runs.

High-signal keys (reuse these names so queries stay consistent):

- `pr_url`
- `pr_number`
- `pipeline_status`
- `deploy_url`
- `external_issue_url`
- `waiting_on`
- `blocked_reason`
- `decision`

Not metadata: logs, summaries, files touched, timestamps, attempt counts,
investigation notes. Those belong in the result comment.

```bash
multica issue metadata set <issue-id> --key pr_url --value <url>
multica issue metadata delete <issue-id> --key <stale-key>
```

`--value` is JSON-parsed by default (bool/number are sniffed); pass `--type
string|number|bool` to force a type.

## Status changes have server side effects

A status change is not cosmetic — the server enqueues or skips agent work based
on it. These are the contracts, not advice:

- **`backlog`** parks an agent-assigned issue: the assignee is set but no task
  fires. Moving `backlog → todo` (or any non-done/non-cancelled status) enqueues
  the assigned agent then.
- **`in_review`** is an accepted issue status. Some workflows use it while a PR
  is open and awaiting review; moving to it is an explicit mutation.
- **`done`** on a child issue posts a system comment on its parent. If a PR
  carries close intent (`Closes MUL-XXXX`), it advances the issue to `done`
  itself on merge — you do not also need to flip it manually.
- **`cancelled`** stops outstanding work; treat it as a user-driven decision.

## Sub-issues: `todo` starts work now, `backlog` parks it

On an agent-assigned issue, create status decides whether the assignee fires
immediately. A non-backlog status (e.g. `todo`) enqueues the agent at create
time; `backlog` sets the assignee without triggering.

Parallel children — all start now:

```bash
multica issue create --title "..." --parent <issue-id> --assignee <agent> --status todo
```

Strictly serial children — park later steps, promote one at a time:

```bash
multica issue create --title "Step 2: ..." --parent <issue-id> --assignee <agent> --status backlog
multica issue status <child-id> todo   # promote when the previous step is truly done
```

Creating every serial step as `todo` enqueues the whole chain at once.

### Stages: order sub-issues into barrier groups

`--stage <N>` (N ≥ 1) groups sub-issues under the same parent into ordered
stages. The parent assignee is woken **once, when a whole stage finishes** —
i.e. every sub-issue in the lowest unfinished stage has reached a terminal
status (`done`/`cancelled`). A completion that does not close a stage is silent
(no comment, no wake). A sibling set with **no** stages is one implicit stage,
so the parent is woken once when the *last* sub-issue finishes — not on every
child.

Advancement is agent-driven: the server only detects the closed barrier and
wakes the parent assignee, who then decides whether to promote the next stage's
`backlog` sub-issues to `todo`.

```bash
# Stage 1 runs now; later stages parked until promoted
multica issue create --title "Research A" --parent <id> --assignee <agent> --stage 1 --status todo
multica issue create --title "Research B" --parent <id> --assignee <agent> --stage 1 --status todo
multica issue create --title "Build"      --parent <id> --assignee <agent> --stage 2 --status backlog
multica issue create --title "Ship"       --parent <id> --assignee <agent> --stage 3 --status backlog
```

When both Stage 1 sub-issues finish you (the parent assignee) are woken with a
"Stage 1 complete" comment. Inspect the layout, then promote the next stage:

```bash
multica issue children <parent-id>             # sub-issues grouped by stage
multica issue status <stage-2-child-id> todo   # promote when its deps are met
```

Read each sub-issue's description before promoting and only promote items whose
stated dependencies are met; if a description conflicts with the parent's
breakdown, leave it `backlog` and comment to confirm first.

## Incorrect → correct

PR title (link the issue):

```text
Fix login redirect                  # incorrect — no issue key, won't link
MUL-2759: fix login redirect        # correct — links the PR
```

Serial / phased sub-issues (don't start the whole chain at once):

```bash
# incorrect — all fire immediately, no ordering
multica issue create --title "Step 2" --parent <issue-id> --assignee <agent> --status todo
multica issue create --title "Step 3" --parent <issue-id> --assignee <agent> --status todo

# correct — stage them; Stage 1 runs, later stages park and are promoted as
# each stage's barrier closes
multica issue create --title "Step 1" --parent <issue-id> --assignee <agent> --stage 1 --status todo
multica issue create --title "Step 2" --parent <issue-id> --assignee <agent> --stage 2 --status backlog
multica issue create --title "Step 3" --parent <issue-id> --assignee <agent> --stage 3 --status backlog
```

## References

`references/working-on-issues-source-map.md` — accurate `file:line` for every
contract above: the `pull-requests` CLI and route, the PR response field list,
`derivePRState`, the two-path link (`extractIdentifiers`) vs close-intent
(`extractClosingIdentifiers`) proof, the backlog enqueue lines, child-done
notify, the stage column / `stageBarrierClosed` barrier and the `--stage` /
`issue children` CLI, and the metadata CLI. Re-derive before depending on an
exact line.
$skill16$,
 '{"origin": {"type": "builtin"}}');

-- multica-working-on-issues: references/working-on-issues-source-map.md
INSERT INTO skill_file (skill_id, path, content) VALUES
((SELECT id FROM skill WHERE skill_type = 'builtin' AND name = 'multica-working-on-issues'),
 'references/working-on-issues-source-map.md',
 $file16$# working-on-issues source map

Evidence layer for `SKILL.md`. Every contract the skill states is traced to a
current `file:line` here. Lines were re-derived against `feat/builtin-skills`
after the latest `main` merge; the prior skill cited pre-merge lines that have
since moved (see the "drifted" column). Re-confirm with the verification command
at the bottom before relying on an exact line.

## `multica issue pull-requests` — read PR links from Multica

| Behavior | File:line | Drifted from |
|---|---|---|
| CLI command `pull-requests <id>` (alias `prs`) | `server/cmd/multica/cmd_issue.go:105` | `:104` |
| `runIssuePullRequests` handler | `server/cmd/multica/cmd_issue.go:507` | new citation |
| Calls `GET /api/issues/<id>/pull-requests` | `server/cmd/multica/cmd_issue.go:522` | `:522` (unchanged) |
| API route registration | `server/cmd/server/router.go:480` | `:480` (unchanged) |
| Handler `ListPullRequestsForIssue` → `Queries.ListPullRequestsByIssue` | `server/internal/handler/github.go:466,471` | `:466` (unchanged) |
| Row → response mapper `issuePullRequestRowToResponse` | `server/internal/handler/github.go:149` | new citation |

The CLI resolves the issue ref, GETs the endpoint, and (for `--output json`)
prints the raw `{"pull_requests": [...]}` body. Only `--output` is accepted; the
default `table` shows `NUMBER STATE TITLE URL`.

## PR response shape

`GitHubPullRequestResponse` struct: `server/internal/handler/github.go:51`. JSON
fields the agent can read off each element of `pull_requests`:

- `number` (`json:"number"`, line 56)
- `html_url` (`json:"html_url"`, line 59)
- `title` (`json:"title"`, line 57)
- `state` (`json:"state"`, line 58) — the folded lifecycle enum (see below)
- `merged_at` (`json:"merged_at"`, line 63), `closed_at` (line 64)
- `mergeable_state` (`json:"mergeable_state"`, line 70) — mirrors GitHub; UI only
  surfaces `clean`/`dirty`, other values round-trip as unknown
- `checks_conclusion` (`json:"checks_conclusion"`, line 74) — aggregated
  `"passed"`/`"failed"`/`"pending"` or `null` (no observed suite)
- `checks_passed` / `checks_failed` / `checks_pending` (lines 78-80) — per-suite
  counts; `aggregateChecksConclusion` (line 183) folds them into
  `checks_conclusion`

There is **no** standalone `draft` or `merged` boolean in the response. The
PR lifecycle is encoded in the single `state` string by `derivePRState`
(`server/internal/handler/github.go:994`):

```
merged   → if PullRequest.Merged
closed   → else if PullRequest.State == "closed"
draft    → else if PullRequest.Draft
open     → otherwise
```

`derivePRState` is called when the webhook upserts the row
(`server/internal/handler/github.go:682`), so `state` is what the list endpoint
returns. "Is it merged?" = `state == "merged"` (or `merged_at != null`); "is it a
draft?" = `state == "draft"`. Combine with `checks_conclusion` for CI status.

## Two distinct webhook paths: link vs close-intent

Both run inside the `pull_request` webhook handler, gated by the workspace
auto-link flag (`workspaceAutoLinkPRsEnabled`, `github.go:1074`).

### Path 1 — link (title OR body OR branch)

- `extractIdentifiers` regex helper: `server/internal/handler/github.go:1028`
- driving regex `identifierRe` (`\b([a-z][a-z0-9]{1,9})-(\d+)\b`, case-insensitive):
  `server/internal/handler/github.go:490`
- call site: `server/internal/handler/github.go:727` —
  `extractIdentifiers(p.PullRequest.Title, p.PullRequest.Body, p.PullRequest.Head.Ref)`

Every `PREFIX-NUMBER` mention in **title, body, or branch** resolves to an issue
in the workspace and writes a link row (`LinkIssueToPullRequest`, ~`github.go:762`).
This is what `multica issue pull-requests` later reads back.

Drifted from the prior skill's `github.go:727` citation, which pointed at the old
call-site location for the link logic.

### Path 2 — close intent (title OR body only, keyword-adjacent)

- `extractClosingIdentifiers` regex helper: `server/internal/handler/github.go:1051`
- driving regex `closingIdentifierRe`
  (`\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)[:\s]+([a-z][a-z0-9]{1,9})-(\d+)\b`):
  `server/internal/handler/github.go:501`
- call site: `server/internal/handler/github.go:736` —
  `extractClosingIdentifiers(p.PullRequest.Title, p.PullRequest.Body)` (no branch arg)

Only a `PREFIX-NUMBER` immediately after a closing keyword
(`Closes`/`Fixes`/`Resolves`, optional `:` then whitespace) sets the link row's
`close_intent` flag — the gate that auto-advances the issue to `done` on merge.
`Fix MUL-1` closes; `Fix login MUL-1` does not (adjacency). Branch names are
deliberately excluded (function doc, `github.go:1044-1050`): a branch like
`mul-1/fix-login` links but must never declare close intent.

Drifted from the prior skill's `github.go:736` citation.

Net: a bare title prefix (`MUL-2759: ...`) or a branch ref links only;
`Closes MUL-2759` links **and** records close intent.

## Status side effects (enqueue contracts)

| Behavior | File:line | Drifted from |
|---|---|---|
| Create-time: agent-assigned, non-backlog issue enqueues immediately | `server/internal/handler/issue.go:2263-2264` | new citation |
| `shouldEnqueueAgentTask` returns false for `backlog` (parking lot) | `server/internal/handler/issue.go:2644-2648` | new citation |
| Backlog → non-backlog (not done/cancelled) enqueues on update | `server/internal/handler/issue.go:2537-2540` | `:2523` |
| Same contract in batch update | `server/internal/handler/issue.go:3021-3024` | new citation |
| Child → `done` notifies + wakes the parent, gated by the stage barrier | `server/internal/handler/issue_child_done.go:66` (`notifyParentOfChildDone`; doc comment at `:15`; barrier gate at `:115`) | func def `:51` |

Creation with `--status todo` (or any non-backlog status) on an agent-assigned
issue fires the agent immediately; `--status backlog` parks it with the assignee
set but no trigger. Promoting `backlog → todo` later fires it then (update path,
line 2537).

## Sub-issue stages (barrier wake)

| Behavior | File:line |
|---|---|
| `issue.stage` column (nullable, `>= 1`) | `server/migrations/123_issue_stage.up.sql` |
| Stage barrier: notify+wake fire only when the lowest unfinished stage is all-terminal; unstaged set = one implicit stage | `server/internal/handler/issue_child_done.go:231` (`stageBarrierClosed`) |
| Per-stage summary + next stage for the wake comment | `server/internal/handler/issue_child_done.go:254` (`stageProgressSummary`) |
| `--stage` on `issue create` / `issue update` | `server/cmd/multica/cmd_issue.go:328,350` |
| `multica issue children <id>` (sub-issues grouped by stage) | `server/cmd/multica/cmd_issue.go:114,678`; route `GET /api/issues/{id}/children` → `ListChildIssues` |

Advancement is agent-driven: the server only detects the closed barrier and
wakes the parent assignee. Promoting the next stage's `backlog` sub-issues to
`todo` is the woken agent's decision, not a server side effect.

## Metadata CLI

| Behavior | File:line |
|---|---|
| `multica issue metadata set <issue-id> --key --value [--type]` | `server/cmd/multica/cmd_issue_metadata.go:80,109-111` |
| `multica issue metadata delete <issue-id> --key` | `server/cmd/multica/cmd_issue_metadata.go:93,113` |
| API routes (PUT/DELETE `/metadata/{key}`) | `server/cmd/server/router.go:478-479` |

`--value` is JSON-parsed by default (bool/number sniff); `--type` forces
`string`/`number`/`bool`.

## Verification command

Re-derive any line above before depending on it:

```bash
cd server
grep -n 'pull-requests <id>'                 cmd/multica/cmd_issue.go
grep -n 'ListPullRequestsForIssue'           cmd/server/router.go internal/handler/github.go
grep -n 'func issuePullRequestRowToResponse\|type GitHubPullRequestResponse struct\|func derivePRState\|func extractIdentifiers\|func extractClosingIdentifiers\|closingIdentifierRe' internal/handler/github.go
grep -n 'extractIdentifiers(\|extractClosingIdentifiers(\|derivePRState(' internal/handler/github.go
grep -n 'prevIssue.Status == "backlog"\|func (h \*Handler) shouldEnqueueAgentTask' internal/handler/issue.go
grep -n 'func notifyParentOfChildDone'       internal/handler/issue_child_done.go
```
$file16$);

