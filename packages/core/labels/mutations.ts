import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { labelKeys } from "./queries";
import { useWorkspaceId } from "../hooks";
import { issueKeys } from "../issues/queries";
import { onIssueLabelsChanged } from "../issues/ws-updaters";
import type {
  Label,
  CreateLabelRequest,
  UpdateLabelRequest,
  ListLabelsResponse,
  IssueLabelsResponse,
} from "../types";

export function useCreateLabel() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateLabelRequest) => api.createLabel(data),
    onSuccess: (label) => {
      const resourceType = label.resource_type || "issue";
      qc.setQueryData<ListLabelsResponse>(
        labelKeys.list(wsId, resourceType),
        (old) =>
          old && !old.labels.some((l) => l.id === label.id)
            ? { ...old, labels: [...old.labels, label], total: old.total + 1 }
            : old,
      );
    },
    onSettled: (_err, _vars, data) => {
      const resourceType = data?.resource_type || "issue";
      qc.invalidateQueries({ queryKey: labelKeys.list(wsId, resourceType) });
    },
  });
}

/**
 * Optimistic rename/recolor. Matches the useUpdateProject pattern: apply the
 * change locally, snapshot for rollback, invalidate on settle. Without this
 * the UI freezes for the round-trip on every edit.
 */
export function useUpdateLabel() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({
      id,
      resource_type: _rt,
      ...data
    }: { id: string; resource_type?: string } & UpdateLabelRequest) =>
      api.updateLabel(id, data),
    onMutate: async ({ id, resource_type, ...data }) => {
      const rt = resource_type || "issue";
      await qc.cancelQueries({ queryKey: labelKeys.list(wsId, rt) });
      const prevList = qc.getQueryData<ListLabelsResponse>(
        labelKeys.list(wsId, rt),
      );
      qc.setQueryData<ListLabelsResponse>(labelKeys.list(wsId, rt), (old) =>
        old
          ? {
              ...old,
              labels: old.labels.map((l) =>
                l.id === id ? { ...l, ...data } : l,
              ),
            }
          : old,
      );
      return { prevList, rt };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevList)
        qc.setQueryData(labelKeys.list(wsId, ctx.rt), ctx.prevList);
    },
    onSettled: () => {
      // Invalidate the entire labels scope so any byIssue cache holding a
      // stale copy of this label is refetched. The list cache is the source
      // of truth; byIssue views will re-render with the fresh data.
      qc.invalidateQueries({ queryKey: labelKeys.all(wsId) });
      // Issues now embed labels (denormalized snapshot), so a rename/recolor
      // also has to refresh the issues caches that hold those snapshots.
      qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
    },
  });
}

export function useDeleteLabel() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id }: { id: string; resource_type?: string }) =>
      api.deleteLabel(id),
    onMutate: async ({ id, resource_type }) => {
      const rt = resource_type || "issue";
      await qc.cancelQueries({ queryKey: labelKeys.list(wsId, rt) });
      const prev = qc.getQueryData<ListLabelsResponse>(
        labelKeys.list(wsId, rt),
      );
      qc.setQueryData<ListLabelsResponse>(labelKeys.list(wsId, rt), (old) =>
        old
          ? {
              ...old,
              labels: old.labels.filter((l) => l.id !== id),
              total: old.total - 1,
            }
          : old,
      );
      return { prev, rt };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev)
        qc.setQueryData(labelKeys.list(wsId, ctx.rt), ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: labelKeys.all(wsId) });
      // A deleted label still lives in cached issue.labels arrays until we
      // refetch — invalidate so list/board chips drop the orphan.
      qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
    },
  });
}

export function useAttachLabel(issueId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (labelId: string) => api.attachLabel(issueId, labelId),
    onMutate: async (labelId) => {
      await qc.cancelQueries({ queryKey: labelKeys.byIssue(wsId, issueId) });
      const prev = qc.getQueryData<IssueLabelsResponse>(
        labelKeys.byIssue(wsId, issueId),
      );
      // Only patch when we already know the current label set — otherwise
      // appending `[label]` to an empty array would wipe denormalized
      // labels in issue list/detail caches and rollback couldn't restore
      // them. If byIssue isn't cached yet (user clicked before the picker
      // fetched), skip the optimistic patch and rely on onSettled refetch.
      if (!prev) return { prev };
      if (prev.labels.some((l) => l.id === labelId)) return { prev };
      const list = qc.getQueryData<ListLabelsResponse>(
        labelKeys.list(wsId, "issue"),
      );
      const label = list?.labels.find((l) => l.id === labelId);
      if (!label) return { prev };
      const next: IssueLabelsResponse = {
        ...prev,
        labels: [...prev.labels, label],
      };
      qc.setQueryData<IssueLabelsResponse>(
        labelKeys.byIssue(wsId, issueId),
        next,
      );
      onIssueLabelsChanged(qc, wsId, issueId, next.labels);
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) {
        qc.setQueryData(labelKeys.byIssue(wsId, issueId), ctx.prev);
        onIssueLabelsChanged(qc, wsId, issueId, ctx.prev.labels);
      }
    },
    onSuccess: (data: IssueLabelsResponse) => {
      // Backend may return an empty object when the post-mutation read fails
      // (it logs a warning and skips the broadcast). Only apply the list
      // when the backend gave us one — otherwise the optimistic patch from
      // onMutate stands until onSettled's invalidation refetches.
      if (data && Array.isArray(data.labels)) {
        qc.setQueryData<IssueLabelsResponse>(
          labelKeys.byIssue(wsId, issueId),
          data,
        );
        onIssueLabelsChanged(qc, wsId, issueId, data.labels);
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: labelKeys.byIssue(wsId, issueId) });
    },
  });
}

export function useDetachLabel(issueId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (labelId: string) => api.detachLabel(issueId, labelId),
    onMutate: async (labelId) => {
      await qc.cancelQueries({ queryKey: labelKeys.byIssue(wsId, issueId) });
      const prev = qc.getQueryData<IssueLabelsResponse>(
        labelKeys.byIssue(wsId, issueId),
      );
      const next = prev
        ? {
            ...prev,
            labels: prev.labels.filter((l: Label) => l.id !== labelId),
          }
        : undefined;
      if (next) {
        qc.setQueryData<IssueLabelsResponse>(
          labelKeys.byIssue(wsId, issueId),
          next,
        );
        onIssueLabelsChanged(qc, wsId, issueId, next.labels);
      }
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) {
        qc.setQueryData(labelKeys.byIssue(wsId, issueId), ctx.prev);
        onIssueLabelsChanged(qc, wsId, issueId, ctx.prev.labels);
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: labelKeys.byIssue(wsId, issueId) });
    },
  });
}

// Agent label mutations

export function useAttachAgentLabel(agentId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (labelId: string) => api.attachLabelToAgent(agentId, labelId),
    onMutate: async (labelId) => {
      await qc.cancelQueries({ queryKey: labelKeys.byAgent(wsId, agentId) });
      const prev = qc.getQueryData<{ labels: Label[] }>(
        labelKeys.byAgent(wsId, agentId),
      );
      if (!prev) return { prev };
      if (prev.labels.some((l) => l.id === labelId)) return { prev };
      const list = qc.getQueryData<ListLabelsResponse>(
        labelKeys.list(wsId, "agent"),
      );
      const label = list?.labels.find((l) => l.id === labelId);
      if (!label) return { prev };
      const next = { labels: [...prev.labels, label] };
      qc.setQueryData(labelKeys.byAgent(wsId, agentId), next);
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev)
        qc.setQueryData(labelKeys.byAgent(wsId, agentId), ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: labelKeys.byAgent(wsId, agentId) });
    },
  });
}

export function useDetachAgentLabel(agentId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (labelId: string) => api.detachLabelFromAgent(agentId, labelId),
    onMutate: async (labelId) => {
      await qc.cancelQueries({ queryKey: labelKeys.byAgent(wsId, agentId) });
      const prev = qc.getQueryData<{ labels: Label[] }>(
        labelKeys.byAgent(wsId, agentId),
      );
      const next = prev
        ? { labels: prev.labels.filter((l) => l.id !== labelId) }
        : undefined;
      if (next)
        qc.setQueryData(labelKeys.byAgent(wsId, agentId), next);
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev)
        qc.setQueryData(labelKeys.byAgent(wsId, agentId), ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: labelKeys.byAgent(wsId, agentId) });
    },
  });
}

// Skill label mutations

export function useAttachSkillLabel(skillId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (labelId: string) => api.attachLabelToSkill(skillId, labelId),
    onMutate: async (labelId) => {
      await qc.cancelQueries({ queryKey: labelKeys.bySkill(wsId, skillId) });
      const prev = qc.getQueryData<{ labels: Label[] }>(
        labelKeys.bySkill(wsId, skillId),
      );
      if (!prev) return { prev };
      if (prev.labels.some((l) => l.id === labelId)) return { prev };
      const list = qc.getQueryData<ListLabelsResponse>(
        labelKeys.list(wsId, "skill"),
      );
      const label = list?.labels.find((l) => l.id === labelId);
      if (!label) return { prev };
      const next = { labels: [...prev.labels, label] };
      qc.setQueryData(labelKeys.bySkill(wsId, skillId), next);
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev)
        qc.setQueryData(labelKeys.bySkill(wsId, skillId), ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: labelKeys.bySkill(wsId, skillId) });
    },
  });
}

export function useDetachSkillLabel(skillId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (labelId: string) => api.detachLabelFromSkill(skillId, labelId),
    onMutate: async (labelId) => {
      await qc.cancelQueries({ queryKey: labelKeys.bySkill(wsId, skillId) });
      const prev = qc.getQueryData<{ labels: Label[] }>(
        labelKeys.bySkill(wsId, skillId),
      );
      const next = prev
        ? { labels: prev.labels.filter((l) => l.id !== labelId) }
        : undefined;
      if (next)
        qc.setQueryData(labelKeys.bySkill(wsId, skillId), next);
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev)
        qc.setQueryData(labelKeys.bySkill(wsId, skillId), ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: labelKeys.bySkill(wsId, skillId) });
    },
  });
}
