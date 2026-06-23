import { z } from "zod";

// ── Space ──

export const WikiSpaceSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  slug: z.string(),
  display_name: z.string(),
  access_scope: z.enum(["shared", "personal"]),
  status: z.enum(["active", "archived"]),
  default_agent_id: z.string().nullable(),
  template: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});

export type WikiSpace = z.infer<typeof WikiSpaceSchema>;

// ── Page ──

export const LinkInfoSchema = z.object({
  target: z.string(),
  title: z.string().nullable(),
  snippet: z.string().nullable(),
  exists: z.boolean(),
});

export const BacklinkInfoSchema = z.object({
  source: z.string(),
  title: z.string().nullable(),
  context: z.string().nullable(),
});

export const WikiPageSchema = z.object({
  id: z.string(),
  space_id: z.string(),
  path: z.string(),
  title: z.string().nullable(),
  page_type: z.string().nullable(),
  content: z.string(),
  content_hash: z.string(),
  validation_warnings: z.array(z.string()),
  created_at: z.string(),
  updated_at: z.string(),
});

export const WikiPageDetailSchema = WikiPageSchema.extend({
  links: z.array(LinkInfoSchema),
  backlinks: z.array(BacklinkInfoSchema),
});

export type WikiPage = z.infer<typeof WikiPageSchema>;
export type WikiPageDetail = z.infer<typeof WikiPageDetailSchema>;
export type LinkInfo = z.infer<typeof LinkInfoSchema>;
export type BacklinkInfo = z.infer<typeof BacklinkInfoSchema>;

// ── Source ──

export const WikiSourceSchema = z.object({
  id: z.string(),
  space_id: z.string(),
  source_type: z.string(),
  title: z.string(),
  url: z.string().nullable(),
  raw_path: z.string(),
  content: z.string(),
  content_hash: z.string(),
  attachment_id: z.string().nullable(),
  mime_type: z.string().nullable(),
  status: z.enum(["captured", "ingested", "archived"]),
  created_at: z.string(),
});

export type WikiSource = z.infer<typeof WikiSourceSchema>;

// ── Operation ──

export const WikiOperationSchema = z.object({
  id: z.string(),
  space_id: z.string(),
  operation_type: z.enum(["ingest", "query", "lint", "distill", "index"]),
  status: z.enum(["pending", "running", "completed", "failed"]),
  hidden_issue_id: z.string().nullable(),
  agent_session_id: z.string().nullable(),
  run_ids: z.array(z.string()),
  cost_cents: z.number(),
  warnings: z.array(z.string()),
  affected_pages: z.array(z.string()),
  created_at: z.string(),
  updated_at: z.string(),
});

export type WikiOperation = z.infer<typeof WikiOperationSchema>;

// ── Request types ──

export interface CreateWikiSpaceRequest {
  slug?: string;
  display_name: string;
  access_scope?: "shared" | "personal";
  template?: string;
}

export interface UpdateWikiSpaceRequest {
  display_name?: string;
  default_agent_id?: string;
}

export interface WriteWikiPageRequest {
  content: string;
  expected_hash?: string;
  summary?: string;
}

export interface BatchReadWikiPagesRequest {
  paths: string[];
}

export interface BatchWriteWikiPageRequest {
  path: string;
  content: string;
  summary?: string;
}

export interface BatchWriteWikiPagesRequest {
  pages: BatchWriteWikiPageRequest[];
}

export interface CreateWikiSourceRequest {
  source_type?: string;
  title: string;
  content: string;
  url?: string;
  raw_path?: string;
}

export interface CreateWikiOperationRequest {
  operation_type: "ingest" | "query" | "lint";
  title?: string;
  prompt?: string;
  source_id?: string;
}

export interface ListWikiPagesParams {
  search?: string;
  page_type?: string;
}

export interface ListWikiOperationsParams {
  limit?: number;
}

export interface CrawlURLRequest { url: string; }
export interface CrawlURLResponse { content: string; url: string; }
