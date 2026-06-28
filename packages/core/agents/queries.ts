import { queryOptions, useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type { CreateAgentTemplateRequest, UpdateAgentTemplateRequest } from "../types";

export const agentTaskSnapshotKeys = {
  all: (wsId: string) => ["workspaces", wsId, "agent-task-snapshot"] as const,
  list: (wsId: string) => [...agentTaskSnapshotKeys.all(wsId), "list"] as const,
};

export const agentActivityKeys = {
  all: (wsId: string) => ["workspaces", wsId, "agent-activity"] as const,
  last30d: (wsId: string) => [...agentActivityKeys.all(wsId), "30d"] as const,
};

export const agentRunCountsKeys = {
  all: (wsId: string) => ["workspaces", wsId, "agent-run-counts"] as const,
  last30d: (wsId: string) => [...agentRunCountsKeys.all(wsId), "30d"] as const,
};

// Workspace-scoped agent task snapshot — every active task plus each agent's
// most recent terminal task. This is the single shared source of truth that
// powers per-agent presence derivation across the app. One fetch per
// workspace; all agent dots / hover cards / list rows derive presence from
// this cache with zero additional network traffic.
//
// The 30s staleTime is a safety net only; the primary freshness signal is
// WS task events, which invalidate this query immediately. Without WS,
// presence still updates within 30s on focus / mount.
export function agentTaskSnapshotOptions(wsId: string) {
  return queryOptions({
    queryKey: agentTaskSnapshotKeys.list(wsId),
    queryFn: () => api.getAgentTaskSnapshot(),
    staleTime: 30 * 1000,
    gcTime: 5 * 60 * 1000,
    refetchOnWindowFocus: true,
  });
}

// Workspace-wide daily task activity for the last 30 days, anchored on
// completed_at. One fetch backs both the Agents-list sparkline (which
// only uses the trailing 7 buckets via `summarizeActivityWindow`) and
// the agent detail "Last 30 days" panel. WS task lifecycle events
// invalidate this query in useRealtimeSync; the staleTime is a
// tab-focus safety net.
export function agentActivity30dOptions(wsId: string) {
  return queryOptions({
    queryKey: agentActivityKeys.last30d(wsId),
    queryFn: () => api.getWorkspaceAgentActivity30d(),
    staleTime: 60 * 1000,
    gcTime: 5 * 60 * 1000,
    refetchOnWindowFocus: true,
  });
}

// Workspace-wide 30-day run counts for the Agents-list RUNS column. Same
// single-fetch / WS-invalidate pattern as activity24hOptions.
export function agentRunCounts30dOptions(wsId: string) {
  return queryOptions({
    queryKey: agentRunCountsKeys.last30d(wsId),
    queryFn: () => api.getWorkspaceAgentRunCounts(),
    staleTime: 60 * 1000,
    gcTime: 5 * 60 * 1000,
    refetchOnWindowFocus: true,
  });
}

export const agentTasksKeys = {
  all: (wsId: string) => ["workspaces", wsId, "agent-tasks"] as const,
  detail: (wsId: string, agentId: string) =>
    [...agentTasksKeys.all(wsId), agentId] as const,
};

// All tasks for a single agent (the agent detail page consumer). Powers both
// the inspector's 7-day throughput stats and the Tasks tab list — shared so
// they don't fetch twice. WS task events invalidate this via the existing
// task-prefix invalidation in useRealtimeSync.
export function agentTasksOptions(wsId: string, agentId: string) {
  return queryOptions({
    queryKey: agentTasksKeys.detail(wsId, agentId),
    queryFn: () => api.listAgentTasks(agentId),
    staleTime: 30 * 1000,
    gcTime: 5 * 60 * 1000,
    refetchOnWindowFocus: true,
  });
}

// Agent templates — DB-backed, platform-level catalog.
// Cache for 5 minutes (shorter than the old Infinity because admin can mutate).
export const agentTemplateKeys = {
  all: () => ["agent-templates"] as const,
  list: (category?: string, tags?: string) =>
    [...agentTemplateKeys.all(), "list", { category, tags }] as const,
  detail: (id: string) => [...agentTemplateKeys.all(), "detail", id] as const,
};

export function agentTemplateListOptions(category?: string, tags?: string) {
  return queryOptions({
    queryKey: agentTemplateKeys.list(category, tags),
    queryFn: () => api.listAgentTemplates({ category, tags }),
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

export function agentTemplateDetailOptions(id: string) {
  return queryOptions({
    queryKey: agentTemplateKeys.detail(id),
    queryFn: () => api.getAgentTemplate(id),
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

export function useAgentTemplates(category?: string, tags?: string) {
  return useQuery(agentTemplateListOptions(category, tags));
}

export function useAgentTemplate(id: string) {
  return useQuery(agentTemplateDetailOptions(id));
}

// Admin mutations
export function useCreateAgentTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateAgentTemplateRequest) =>
      api.createAgentTemplate(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: agentTemplateKeys.all() });
    },
  });
}

export function useUpdateAgentTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: UpdateAgentTemplateRequest;
    }) => api.updateAgentTemplate(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: agentTemplateKeys.all() });
    },
  });
}

export function useDeleteAgentTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteAgentTemplate(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: agentTemplateKeys.all() });
    },
  });
}

export function usePlatformAdmin() {
  return useQuery({
    queryKey: ["platform-admin"],
    queryFn: () => api.checkPlatformAdmin(),
    staleTime: 5 * 60 * 1000,
  });
}
