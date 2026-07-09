# Chat V2 — IM-style Chat Tab Design

**Date:** 2026-07-09
**Reference:** upstream PR [#5076](https://github.com/multica-ai/multica/pull/5076) (MUL-4171)
**Strategy:** Reimplement on current codebase, referencing PR design. Not cherry-picked from upstream.

## Motivation

Replace the floating chat FAB/window with a first-class **Chat** tab under Inbox, laid out as an IM-style two-pane surface (thread list on the left, conversation on the right — mirroring the Inbox page). The floating chat window is retained as an optional mode.

## Architecture Overview

```
apps/web/app/[workspaceSlug]/(dashboard)/
├── chat/                          ← NEW: Chat route page
│   └── page.tsx                   URL: /:slug/chat?session=<id>
├── layout.tsx                     ← MODIFY: add Chat nav item to sidebar
│
apps/desktop/.../routes.tsx        ← MODIFY: add Chat route for desktop
│
packages/views/
├── chat/
│   ├── chat-page.tsx              ← NEW: dual-pane IM main page
│   ├── floating-chat.tsx           ← NEW: mode orchestrator (tab vs floating)
│   ├── components/
│   │   ├── chat-thread-list.tsx   ← NEW: left-side thread list
│   │   ├── chat-session-header.tsx← NEW: right-side conversation header
│   │   ├── chat-empty-state.tsx   ← NEW: agent-aware empty state
│   │   ├── new-chat-button.tsx    ← NEW: new chat button + agent picker
│   │   ├── quick-agent-bar.tsx    ← NEW: pinned agents horizontal bar (kept, not mounted)
│   │   ├── archived-agent-banner.tsx ← NEW: banner for archived agents
│   │   ├── chat-input.tsx         ← REUSE (existing)
│   │   ├── chat-message-list.tsx  ← REUSE (existing)
│   │   ├── chat-window.tsx        ← KEEP: floating window (minor changes)
│   │   ├── chat-fab.tsx           ← KEEP: FAB button
│   │   └── chat-resize-handles.tsx← REUSE (existing)
│   └── index.ts                   ← MODIFY: export new components
│
├── layout/
│   └── app-sidebar.tsx            ← MODIFY: add Chat nav item below Inbox
│
packages/core/
├── types/chat.ts                  ← MODIFY: add last_read_at, unread_count, agent_status, is_pinned
├── chat/
│   ├── queries.ts                 ← MODIFY: add pinned agent/session queries
│   ├── mutations.ts               ← MODIFY: add pin/rename/unread mutations
│   └── store.ts                   ← MODIFY: add chat_mode, thread list state
│
packages/ui/
├── components/common/             ← REUSE: submit-button, badges, etc.
│
server/
├── internal/handler/
│   ├── chat.go                    ← MODIFY: read cursor, pin, agent_status endpoints
│   ├── chat_pinned_agent.go       ← NEW: agent pin CRUD
│   └── agent.go                   ← MODIFY: best-effort welcome chat on creation
├── internal/service/task.go       ← MODIFY: support chat_intro task type
├── internal/daemon/prompt.go      ← MODIFY: chat_intro system prompt
├── pkg/db/queries/
│   ├── chat.sql                   ← MODIFY: read cursor, pin, unread count queries
│   └── chat_pinned_agent.sql      ← NEW: pinned agent queries
├── pkg/db/generated/              ← REGEN: sqlc after query changes
├── pkg/protocol/messages.go       ← MODIFY: WS event types for pinned changes
└── migrations/
    ├── 146_chat_read_cursor.{up,down}.sql
    ├── 147_chat_pinned_agent.{up,down}.sql
    └── 148_chat_session_pinned.{up,down}.sql
```

### Key Design Decisions

- Chat page uses URL search param `?session=<id>` for session selection (does not pollute route path)
- `chat-page.tsx` is shared by both Web (`page.tsx`) and Desktop (`routes.tsx`), following existing shared package patterns
- Existing components `chat-input.tsx`, `chat-message-list.tsx` are reused directly in the dual-pane layout
- Floating window is retained via `floating-chat.tsx` orchestrator that checks current mode
- Web and Desktop have identical Chat V2 experience

## Implementation Slices

### Slice 1: Chat Tab + Dual-Pane Layout

**Routing:**
- Web: `apps/web/app/[workspaceSlug]/(dashboard)/chat/page.tsx` renders `<ChatPage />`
  - Reads `?session=<id>` from `useSearchParams()`
- Desktop: `apps/desktop/.../routes.tsx` adds `/chat` route rendering `<ChatPage />`

**Sidebar Navigation:**
- `app-sidebar.tsx`: insert Chat nav item below Inbox
- Icon: `MessageSquare`
- Unread count badge placeholder (implemented in Slice 2)

**ChatPage Layout:**
```
┌──────────────────────────────────────────┐
│ Left (w-80)           │ Right (flex-1)   │
│                       │                  │
│ [New Chat +]          │ [Session Header] │
│ ───────────────       │ ───────────────  │
│ Thread List           │ Message Area     │
│ ┌────────────────┐    │ ┌──────────────┐ │
│ │🤖 Agent A      │    │ │ User message  │ │
│ │  Preview...    │    │ │              │ │
│ │  2m ago   🔴2 │    │ │ 🤖 Reply...  │ │
│ │                │    │ │              │ │
│ │🤖 Agent B      │    │ │              │ │
│ │  Hello!        │    │ ├──────────────┤ │
│ │  1h ago        │    │ │ [Input area] │ │
│ └────────────────┘    │ └──────────────┘ │
└──────────────────────────────────────────┘
```

**Thread List (`chat-thread-list.tsx`):**
- Sorted by `updated_at DESC`
- Each row: agent avatar + title (derived from first message) + last message preview + timestamp
- Hover reveals delete/stop buttons (rename lives only in session header ⋯ menu)
- Selected session highlighted
- Reuses existing `listChatSessions` API, adds `unread_count` field

**Empty State (`chat-empty-state.tsx`):**
- Shown when no session is selected
- Agent avatar + "Chat with {name}" + agent description + starter prompt buttons
- Starter prompts from agent config or defaults

**New Chat (`new-chat-button.tsx`):**
- Click opens agent picker (reuses existing `AgentDropdown` logic)
- Selecting an agent creates session → auto-selects it

### Slice 2: Read Cursors + Unread Counts

**Migration `146_chat_read_cursor`:**
```sql
-- Add last_read_at column
ALTER TABLE chat_session ADD COLUMN last_read_at TIMESTAMPTZ;

-- Data migration: existing has_unread=false → mark as read
UPDATE chat_session SET last_read_at = NOW() WHERE has_unread = false;
-- has_unread=true → leave last_read_at as NULL (unread)

-- Index for activity + unread sorting
CREATE INDEX idx_chat_session_creator_activity
  ON chat_session(creator_id, updated_at DESC);
```

**Server changes:**
- `MarkChatSessionRead` PATCH: sets `last_read_at = NOW()`
- `ListChatSessions` response adds `unread_count` field:
  `COUNT(*) FROM chat_message WHERE chat_session_id = s.id AND created_at > COALESCE(last_read_at, '1970-01-01') AND role = 'assistant'`
- `SendChatMessage`: assistant messages no longer set `has_unread`; unread determined by `last_read_at < message.created_at`

**Core layer:**
- `ChatSession` type adds `last_read_at: string | null`, `unread_count: number`
- New hook `useUnreadCountTotal()` aggregates across sessions for sidebar badge

**UI:**
- Thread list row: red count badge when `unread_count > 0`
- Sidebar Chat nav: total unread badge (matching Inbox badge style)
- Entering a session auto-calls `markChatSessionRead`
- WebSocket: incoming `chat:message` for non-active session → increment client-side unread count (optimistic)

### Slice 3: Session Title + Rename + Agent-Aware Empty State

**Title derivation (server-side, in `SendChatMessage`):**
1. Take first `role='user'` message content
2. Strip markdown: `#`, `*`, `**`, `` ` ``, `>`, links, images
3. Collapse whitespace (`\s+` → ` `)
4. Truncate to 30 characters
5. Fallback: `"Chat with {agent_name}"` if no user messages
6. Title computed when `title IS NULL` on first send

**Rename:**
- Entry: session header `⋯` menu → "Rename"
- Interaction: inline edit (click title → input → Enter to confirm → blur to cancel)
- Calls existing `useUpdateChatSession` mutation
- Not available from thread list row (matches PR behavior)

**Agent-Aware Empty State (`chat-empty-state.tsx`):**
```
┌─────────────────────────────────┐
│        🤖 (agent avatar)        │
│    Chat with CEO Agent          │
│  I am the team CEO, responsible │
│  for task analysis, delegation, │
│  and team management.           │
│  ┌─────────────────────────┐   │
│  │ 💡 What can you do?     │   │
│  └─────────────────────────┘   │
│  ┌─────────────────────────┐   │
│  │ 📋 Help me plan a task  │   │
│  └─────────────────────────┘   │
└─────────────────────────────────┘
```
- Starter prompts from agent template or hardcoded defaults
- Clicking a starter prompt auto-fills and sends

### Slice 4: Agent/Session Pinning

**Migration `147_chat_pinned_agent`:**
```sql
CREATE TABLE chat_pinned_agent (
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    sort_order SMALLINT NOT NULL DEFAULT 0 CHECK (sort_order >= 0 AND sort_order <= 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, agent_id, workspace_id)
);
-- Max 5 per user per workspace enforced in application layer
```

**Migration `148_chat_session_pinned`:**
```sql
ALTER TABLE chat_session ADD COLUMN is_pinned BOOLEAN NOT NULL DEFAULT false;
```

**Handlers (`chat_pinned_agent.go`):**
- `POST /api/workspaces/{id}/chat/pinned-agents` — pin agent (validates COUNT < 5)
- `DELETE /api/workspaces/{id}/chat/pinned-agents/{agentId}` — unpin
- `GET /api/workspaces/{id}/chat/pinned-agents` — list pinned agents
- `PATCH /api/workspaces/{id}/chat/sessions/{id}/pin` — toggle session pin

**Core mutations (optimistic):**
- `usePinnedAgents()` query
- `usePinAgent()`, `useUnpinAgent()` mutations
- `useToggleSessionPin()` mutation

**Thread list sort order:**
1. Pinned sessions (`is_pinned=true`) first
2. Within each group, by `updated_at DESC`
3. Divider between pinned/unpinned sections

**Pin entry points:**
- Agent pin: new-chat agent picker shows 📌 on pinned agents, right-click to toggle
- Session pin: session header `⋯` menu → "Pin to top" / "Unpin"
- Quick agent bar (`quick-agent-bar.tsx`): horizontal pinned agent avatars above thread list (kept in tree, not mounted initially)

### Slice 5: Welcome Chat + Archived Banner

**No migration needed** — welcome chat uses existing `chat_session` and `agent_task_queue` tables.

**Welcome Chat Flow:**
```
POST /api/agents (create agent)
  → agent record created
  → async best-effort:
     1. Create chat_session (agent_id = new agent, creator = current user)
     2. Enqueue agent_task:
        - task_type: 'chat_intro'
        - system prompt: "Introduce yourself briefly to a new user..."
     3. On LLM/daemon failure → silent skip, agent creation unblocked
     4. On success → assistant message written → session.updated_at updated
```

**Key behaviors:**
- Uses existing `CreateChatSession` + task enqueue infrastructure
- `role='assistant'` message written directly (no user message)
- If task fails/timeout/daemon offline: session exists but empty → user sees empty state on open
- First assistant intro message does not trigger unread (it's from assistant, user hasn't opened the chat)

**Archived Agent Banner (`archived-agent-banner.tsx`):**
```
┌────────────────────────────────────────────┐
│  ⚠️ Agent "{name}" has been archived.       │
│  You can still read this conversation.      │
│                              [Start New]    │
└────────────────────────────────────────────┘
```
- Shown above message area when `chat_session.agent_status = 'archived'`
- Input disabled
- "Start New" opens agent picker for a different agent
- `GetChatSession` response gains `agent_status` field (JOIN agent table)

### Slice 6: Floating Window Retention + Mode Toggle

**Mode orchestration (`floating-chat.tsx`):**
- Reads `chat_mode: 'tab' | 'floating'` from Zustand (persisted to localStorage)
- `chat_mode === 'floating'`: render `<ChatFab />` + `<ChatWindow />` (existing behavior)
- `chat_mode === 'tab'`: FAB hidden, Chat tab handles display
- User can switch back to floating from ChatPage header

**ChatWindow changes (minimal):**
- Thread list extracted as standalone `chat-thread-list.tsx` component
- ChatWindow in floating mode embeds thread list internally (via session dropdown)
- ChatWindow body reuses `chat-message-list.tsx` + `chat-input.tsx` as before

**Settings entry:**
- `packages/views/settings/components/chat-tab.tsx`:
  ```
  Chat display mode:  ○ Tab (sidebar)   ● Floating (popup)
  ```
- Default: `'tab'` (Chat V2 new default)
- First-time detection: if user had active floating chat sessions, show migration hint

**Constraints:**
- Two modes cannot be active simultaneously
- `useChatStore.activeSessionId` shared across modes — switching preserves current session
- Desktop: both modes supported, settings independent per platform

## Database Migrations Summary

| # | Name | Purpose |
|---|------|---------|
| 146 | `chat_read_cursor` | Add `last_read_at` column, migrate existing `has_unread` data |
| 147 | `chat_pinned_agent` | New table for per-user pinned agents (max 5) |
| 148 | `chat_session_pinned` | Add `is_pinned` boolean to `chat_session` |

## API Changes Summary

| Method | Endpoint | Purpose | Slice |
|--------|----------|---------|-------|
| GET | `/api/workspaces/{id}/chat/sessions` | Add `unread_count`, `last_read_at` to response | 2 |
| PATCH | `/api/workspaces/{id}/chat/sessions/{id}/read` | Set `last_read_at = NOW()` | 2 |
| PATCH | `/api/workspaces/{id}/chat/sessions/{id}` | Update title (existing, unchanged) | 3 |
| PATCH | `/api/workspaces/{id}/chat/sessions/{id}/pin` | Toggle `is_pinned` | 4 |
| GET | `/api/workspaces/{id}/chat/pinned-agents` | List user's pinned agents | 4 |
| POST | `/api/workspaces/{id}/chat/pinned-agents` | Pin an agent | 4 |
| DELETE | `/api/workspaces/{id}/chat/pinned-agents/{agentId}` | Unpin an agent | 4 |
| GET | `/api/workspaces/{id}/chat/sessions/{id}` | Add `agent_status` to response | 5 |
| POST | `/api/agents` | Add best-effort welcome chat enqueue | 5 |

## New Core Types

```typescript
// Additions to ChatSession
interface ChatSession {
  // ... existing fields
  last_read_at: string | null;
  unread_count: number;
  agent_status: 'active' | 'archived' | 'deleted';
  is_pinned: boolean;
}

// New: Pinned agent
interface PinnedAgent {
  agent_id: string;
  sort_order: number;
  created_at: string;
  agent: { id: string; name: string; avatar_url?: string };
}

// New: Chat mode
type ChatMode = 'tab' | 'floating';
```

## New Zustand State

```typescript
// Additions to ChatStore
interface ChatState {
  // ... existing fields
  chatMode: ChatMode;  // 'tab' | 'floating', persisted
  setChatMode: (mode: ChatMode) => void;
}
```

## Testing Strategy

| Layer | Test Location | What to Test |
|-------|---------------|--------------|
| Migrations | `server/migrations/` | Up/down cycle, data integrity |
| DB queries | sqlc generated | Parameter binding, return types |
| Handlers | `server/internal/handler/chat_test.go` | HTTP status, response shape, auth gates |
| Core queries | `packages/core/chat/queries.test.ts` | Query key structure, cache invalidation |
| Core mutations | `packages/core/chat/*.test.ts` | Optimistic updates, rollback on error |
| Core store | `packages/core/chat/store.test.ts` | Persistence, mode switching |
| Views | `packages/views/chat/components/*.test.tsx` | Rendering, user interactions, empty/error states |
| E2E | `e2e/chat-v2.spec.ts` | Full flow: create session → send → receive → unread → pin |

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Large PR, many files | Vertical slices: each slice is self-contained and testable |
| Breaking existing chat | Floating mode preserved; existing components unchanged |
| Welcome chat LLM cost | Best-effort only; fails silently; one intro per agent creation |
| Unread cursor migration | All existing messages marked read; only new messages use cursor |
| WebSocket sync across modes | Shared Zustand store; mode switch preserves activeSessionId |
