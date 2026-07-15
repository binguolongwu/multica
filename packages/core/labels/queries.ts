import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const labelKeys = {
  all: (wsId: string) => ["labels", wsId] as const,
  list: (wsId: string, resourceType: string) =>
    [...labelKeys.all(wsId), "list", resourceType] as const,
  detail: (wsId: string, id: string) =>
    [...labelKeys.all(wsId), "detail", id] as const,
  byIssue: (wsId: string, issueId: string) =>
    [...labelKeys.all(wsId), "issue", issueId] as const,
  byAgent: (wsId: string, agentId: string) =>
    [...labelKeys.all(wsId), "agent", agentId] as const,
  bySkill: (wsId: string, skillId: string) =>
    [...labelKeys.all(wsId), "skill", skillId] as const,
};

export function labelListOptions(wsId: string, resourceType: string = "issue") {
  return queryOptions({
    queryKey: labelKeys.list(wsId, resourceType),
    queryFn: () => api.listLabels(resourceType),
    select: (data) => data.labels,
  });
}

export function issueLabelsOptions(wsId: string, issueId: string) {
  return queryOptions({
    queryKey: labelKeys.byIssue(wsId, issueId),
    queryFn: () => api.listLabelsForIssue(issueId),
    select: (data) => data.labels,
    enabled: Boolean(issueId),
  });
}

export function agentLabelsOptions(wsId: string, agentId: string) {
  return queryOptions({
    queryKey: labelKeys.byAgent(wsId, agentId),
    queryFn: () => api.listLabelsForAgent(agentId),
    select: (data) => data.labels,
    enabled: Boolean(agentId),
  });
}

export function skillLabelsOptions(wsId: string, skillId: string) {
  return queryOptions({
    queryKey: labelKeys.bySkill(wsId, skillId),
    queryFn: () => api.listLabelsForSkill(skillId),
    select: (data) => data.labels,
    enabled: Boolean(skillId),
  });
}
