import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { wikiKeys } from "./queries";
import type {
  CreateWikiSpaceRequest,
  UpdateWikiSpaceRequest,
  WriteWikiPageRequest,
  CreateWikiSourceRequest,
  CreateWikiOperationRequest,
} from "./types";

export function useCreateWikiSpace(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWikiSpaceRequest) => api.createWikiSpace(wsId, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: wikiKeys.spaces(wsId) }); },
  });
}

export function useUpdateWikiSpace(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ slug, data }: { slug: string; data: UpdateWikiSpaceRequest }) =>
      api.updateWikiSpace(wsId, slug, data),
    onSuccess: (_, { slug }) => {
      qc.invalidateQueries({ queryKey: wikiKeys.spaceDetail(wsId, slug) });
    },
  });
}

export function useUpsertWikiPage(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ path, data }: { path: string; data: WriteWikiPageRequest }) =>
      api.upsertWikiPage(wsId, slug, path, data),
    onSuccess: (_, { path }) => {
      qc.invalidateQueries({ queryKey: wikiKeys.pages(wsId, slug) });
      qc.invalidateQueries({ queryKey: wikiKeys.pageDetail(wsId, slug, path) });
    },
  });
}

export function useDeleteWikiPage(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (path: string) => api.deleteWikiPage(wsId, slug, path),
    onSuccess: () => { qc.invalidateQueries({ queryKey: wikiKeys.pages(wsId, slug) }); },
  });
}

export function useCreateWikiSource(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWikiSourceRequest) => api.createWikiSource(wsId, slug, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: wikiKeys.sources(wsId, slug) }); },
  });
}

export function useCreateWikiOperation(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWikiOperationRequest) => api.createWikiOperation(wsId, slug, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: wikiKeys.operations(wsId, slug) }); },
  });
}
