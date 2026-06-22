import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { ListWikiPagesParams, ListWikiOperationsParams } from "./types";

export const wikiKeys = {
  all: (wsId: string) => ["wiki", wsId] as const,
  spaces: (wsId: string) => [...wikiKeys.all(wsId), "spaces"] as const,
  spaceDetail: (wsId: string, slug: string) =>
    [...wikiKeys.spaces(wsId), slug] as const,
  pages: (wsId: string, slug: string) =>
    [...wikiKeys.all(wsId), "pages", slug] as const,
  pageDetail: (wsId: string, slug: string, path: string) =>
    [...wikiKeys.pages(wsId, slug), path] as const,
  pageRevisions: (wsId: string, slug: string, path: string) =>
    [...wikiKeys.pageDetail(wsId, slug, path), "revisions"] as const,
  sources: (wsId: string, slug: string) =>
    [...wikiKeys.all(wsId), "sources", slug] as const,
  sourceDetail: (wsId: string, slug: string, id: string) =>
    [...wikiKeys.sources(wsId, slug), id] as const,
  operations: (wsId: string, slug: string) =>
    [...wikiKeys.all(wsId), "operations", slug] as const,
  operationDetail: (wsId: string, slug: string, id: string) =>
    [...wikiKeys.operations(wsId, slug), id] as const,
};

export function wikiSpacesOptions(wsId: string) {
  return queryOptions({ queryKey: wikiKeys.spaces(wsId), queryFn: () => api.listWikiSpaces() });
}

export function wikiSpaceDetailOptions(wsId: string, slug: string) {
  return queryOptions({ queryKey: wikiKeys.spaceDetail(wsId, slug), queryFn: () => api.getWikiSpace(slug) });
}

export function wikiPagesOptions(wsId: string, slug: string, params?: ListWikiPagesParams) {
  return queryOptions({
    queryKey: [...wikiKeys.pages(wsId, slug), params ?? {}] as const,
    queryFn: () => api.listWikiPages(slug, params),
  });
}

export function wikiPageDetailOptions(wsId: string, slug: string, path: string) {
  return queryOptions({
    queryKey: wikiKeys.pageDetail(wsId, slug, path),
    queryFn: () => api.getWikiPage(slug, path),
    enabled: !!path,
  });
}

export function wikiSourcesOptions(wsId: string, slug: string) {
  return queryOptions({ queryKey: wikiKeys.sources(wsId, slug), queryFn: () => api.listWikiSources(slug) });
}

export function wikiOperationsOptions(wsId: string, slug: string, params?: ListWikiOperationsParams) {
  return queryOptions({
    queryKey: [...wikiKeys.operations(wsId, slug), params ?? {}] as const,
    queryFn: () => api.listWikiOperations(slug, params),
  });
}
