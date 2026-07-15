/**
 * Labels — workspace-scoped, applied as many-to-many to issues, agents, and skills.
 *
 * Labels are lightweight metadata (name + color) distinct from projects:
 * projects group related work, labels are cross-cutting tags (bug, feature,
 * performance, …). Colors are normalized to lowercase `#RRGGBB`.
 *
 * resource_type determines which entity type the label applies to: "issue",
 * "agent", or "skill".
 */
export type LabelResourceType = "issue" | "agent" | "skill";

export interface Label {
  id: string;
  workspace_id: string;
  name: string;
  /** Normalized lowercase hex color, e.g. `#3b82f6`. */
  color: string;
  resource_type: LabelResourceType;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface CreateLabelRequest {
  name: string;
  color: string;
  resource_type?: LabelResourceType;
  description?: string;
}

export interface UpdateLabelRequest {
  name?: string;
  color?: string;
  description?: string;
}

export interface ListLabelsResponse {
  labels: Label[];
  total: number;
}

export interface IssueLabelsResponse {
  labels: Label[];
}
