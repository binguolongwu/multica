-- Stage 3.1 of the Composio epic (MUL-3869, parent MUL-3721): the per-task
-- Composio MCP overlay is only mounted when the agent's owner has explicitly
-- allowlisted toolkits. The overlay uses the agent OWNER's active Composio
-- connection (MUL-3963), so sharing the agent shares those apps with whoever
-- may invoke it.
--
-- Two columns ship together because the dispatch decision needs both:
--   * agent.composio_toolkit_allowlist - which toolkits the agent is allowed
--     to surface; NULL or [] = no MCP overlay.
--   * agent_task_queue.originator_user_id - the top-of-chain human originator
--     for THIS task, inherited across agent @-mention fanout. Used by
--     canInvokeAgent to judge A2A by the originator and for audit/attribution
--     (the Composio overlay now follows invocation permission and uses the
--     agent owner's connection, so this is no longer an originator == owner
--     gate - MUL-3963).

-- ----------------------------------------------------------------------------
-- agent.composio_toolkit_allowlist
-- ----------------------------------------------------------------------------
ALTER TABLE agent
    ADD COLUMN composio_toolkit_allowlist TEXT[] NULL;

COMMENT ON COLUMN agent.composio_toolkit_allowlist IS
    'Composio toolkit slugs this agent is allowed to mount as MCP. NULL or empty array = no MCP overlay. Mounted for any run that passes the agent invocation-permission gate (MUL-3963); the overlay uses the agent OWNER''s active Composio connection, so sharing the agent (public_to) shares these apps with whoever may invoke it. No longer gated on originator == owner. Stored as TEXT[] so the dispatch path can intersect against the owner''s active connections with a single SQL ANY() filter.';

-- ----------------------------------------------------------------------------
-- agent_task_queue.originator_user_id
-- ----------------------------------------------------------------------------
--
-- Distinct from initiator_user_id (migration 117): initiator is whoever
-- physically sent the triggering message - which for an agent fan-out is
-- ANOTHER AGENT, not a human - and is set today only for chat tasks. The
-- Composio overlay decision needs the HUMAN at the top of the chain so we
-- never inadvertently project a human's connected apps into a run that
-- person did not start. Hence a dedicated column whose contract is "the
-- originator is always a 'user' row (or NULL when none can be attributed,
-- e.g. autopilot/system runs)".
--
-- No FK to "user" by design (matches initiator_user_id and the
-- comment.source_task_id pattern): foreign keys on hot-path queue tables
-- have lock implications on busy production deploys, and the column only
-- feeds best-effort overlay decisions - a stale id simply degrades to "no
-- overlay", never a corrupt run.
ALTER TABLE agent_task_queue
    ADD COLUMN originator_user_id UUID NULL;

COMMENT ON COLUMN agent_task_queue.originator_user_id IS
    'Top-of-chain human originator for this run. For human-triggered tasks (comment by a member, chat, quick-create) equals that member. For agent-fanout tasks inherited from the parent task''s originator_user_id via comment.source_task_id. NULL when no human is in the chain (autopilot, system-driven). Used by canInvokeAgent to judge A2A by the originator; the Composio overlay now follows invocation permission and uses the agent owner''s connection, so this is audit/attribution + A2A gating, NOT a Composio owner==originator gate (MUL-3963).';

-- Dispatch reads originator_user_id for every overlay decision; a partial
-- index on the non-NULL subset keeps the working set small (autopilot and
-- pre-stage-3.1 rows stay out of the index entirely).
CREATE INDEX agent_task_queue_originator_user_id_idx
    ON agent_task_queue(originator_user_id)
    WHERE originator_user_id IS NOT NULL;
