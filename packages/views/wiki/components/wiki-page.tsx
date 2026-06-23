"use client";

import { useState, useCallback, useMemo, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileText, BookOpen, Search, Loader2, Download, ChevronDown, Check } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { wikiSpacesOptions, wikiPagesOptions, wikiPageDetailOptions, useCreateWikiSpace, useUpsertWikiPage, useDeleteWikiPage, useUpdateWikiSpace } from "@multica/core/wiki";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@multica/ui/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { ActorAvatar } from "../../common/actor-avatar";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import { toast } from "sonner";
import { WikiFileTree } from "./wiki-file-tree";
import { WikiPageViewer } from "./wiki-page-viewer";
import { WikiIngestDialog } from "./wiki-ingest-dialog";

const DEFAULT_SPACE = "default";

export function WikiPage() {
  const wsId = useWorkspaceId();
  const { t } = useT("layout");
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [spaceSlug] = useState(DEFAULT_SPACE);
  const [ingestOpen, setIngestOpen] = useState(false);
  const [wikiAgentId, setWikiAgentId] = useState<string>("");
  const [selectedDir, setSelectedDir] = useState<string>("");
  const updateSpace = useUpdateWikiSpace(wsId);

  const { data: spaces, isLoading: spacesLoading } = useQuery(wikiSpacesOptions(wsId));

  // Load saved agent from wiki_space on mount.
  useEffect(() => {
    if (!wikiAgentId && spaces?.[0]?.default_agent_id) {
      setWikiAgentId(spaces[0].default_agent_id);
    }
  }, [spaces, wikiAgentId]);

  const enabledDirs = useMemo(() => {
    const defaultDirs = ["raw", "wiki/entities", "wiki/intents", "wiki/knowledge", "wiki/policies", "wiki/procedures", "wiki/insights", "wiki/summaries"];
    try {
      const s = spaces?.[0];
      if (s && (s as any).settings?.enabled_dirs) {
        const dirs = (s as any).settings.enabled_dirs;
        return ["raw", ...dirs.map((d: string) => `wiki/${d}`)];
      }
    } catch {}
    return defaultDirs;
  }, [spaces]);

  const { data: agents } = useQuery(agentListOptions(wsId));
  const [agentOpen, setAgentOpen] = useState(false);
  const [agentSearch, setAgentSearch] = useState("");
  const filteredAgents = (agents || []).filter((a) =>
    !agentSearch || a.name.toLowerCase().includes(agentSearch.toLowerCase()),
  );
  const selectedAgent = agents?.find((a) => a.id === wikiAgentId);
  const createSpace = useCreateWikiSpace(wsId);
  const upsertPage = useUpsertWikiPage(wsId, spaceSlug);
  const deletePage = useDeleteWikiPage(wsId, spaceSlug);
  const { data: pages, isLoading: pagesLoading } = useQuery(
    wikiPagesOptions(wsId, spaceSlug, searchQuery ? { search: searchQuery } : undefined),
  );
  const { data: pageDetail, isLoading: pageLoading } = useQuery(
    wikiPageDetailOptions(wsId, spaceSlug, selectedPath ?? ""),
  );

  const handleSelectPage = useCallback((path: string) => {
    // Append .md to paths without an extension so [[wiki/entities/foo]] resolves.
    const last = path.split("/").pop() || "";
    const resolved = last.includes(".") ? path : `${path}.md`;
    setSelectedPath(resolved);
    setSearchQuery("");
  }, []);

  const handleCreateDir = useCallback((parentDir: string, name: string) => {
    const dirPath = parentDir ? `${parentDir}/${name}` : name;
    upsertPage.mutate({
      path: `${dirPath}/.gitkeep`,
      data: { content: "." },
    }, {
      onSuccess: () => { setSelectedDir(dirPath); },
      onError: () => toast.error("Failed to create directory"),
    });
  }, [upsertPage]);

  // Protected paths: schema rules, system logs, templates, index, AGENTS, IDEA
  // Protected from deletion: schema, system, templates, index, AGENTS, IDEA
  const isProtectedPath = useCallback((path: string) => {
    const del = [/^schema\//, /^system\//, /\/_TEMPLATE\.md$/, /^wiki\/index\.md$/, /^wiki\/log\.md$/, /^AGENTS\.md$/, /^IDEA\.md$/];
    return del.some((p) => p.test(path));
  }, []);

  // Read-only (not editable): raw/, schema/, system/, AGENTS.md, IDEA.md
  // system/update_log.md is editable (append-only), wiki/* is editable
  const isPageEditable = useCallback((path: string) => {
    const ro = [/^raw\//, /^schema\//, /^system\/(?!update_log\.md)/, /^AGENTS\.md$/, /^IDEA\.md$/];
    return !ro.some((p) => p.test(path));
  }, []);

  const handleDelete = useCallback((targetPath: string, isFile: boolean) => {
    if (isFile) {
      if (isProtectedPath(targetPath)) {
        toast.error(t(($) => $.wiki_page.tree_protected));
        return;
      }
      if (!confirm(`Delete "${targetPath}"? This cannot be undone.`)) return;
      deletePage.mutate(targetPath, {
        onSuccess: () => {
          if (selectedPath === targetPath) setSelectedPath(null);
        },
      });
    } else {
      // Protected directories: schema, system are undeletable
      if (/^(schema|system)$/.test(targetPath)) {
        toast.error(t(($) => $.wiki_page.tree_protected));
        return;
      }
      // Directory: check for children first
      const hasChildren = (pages || []).some((p) =>
        p.path.startsWith(`${targetPath}/`) && !p.path.endsWith("/.gitkeep"),
      );
      if (hasChildren) {
        alert(t(($) => $.wiki_page.tree_cannot_delete));
        return;
      }
      if (!confirm(`Delete directory "${targetPath}/"? This cannot be undone.`)) return;
      // Delete the .gitkeep marker
      const gitkeep = (pages || []).find((p) => p.path === `${targetPath}/.gitkeep`);
      if (gitkeep) deletePage.mutate(gitkeep.path);
      // Also delete any empty subdir .gitkeep markers recursively
      (pages || []).filter((p) => p.path.startsWith(`${targetPath}/`) && p.path.endsWith("/.gitkeep")).forEach((p) => {
        deletePage.mutate(p.path);
      });
      if (selectedDir === targetPath) setSelectedDir("");
    }
  }, [pages, deletePage, selectedPath, selectedDir, t]);

  if (spacesLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!spaces || spaces.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 p-8">
        <BookOpen className="h-12 w-12 text-muted-foreground" />
        <div className="text-center">
          <h2 className="text-lg font-semibold">{t(($) => $.wiki_page.not_setup_title)}</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {t(($) => $.wiki_page.not_setup_desc)}
          </p>
        </div>
        <Button
          onClick={() => createSpace.mutate({ display_name: "default" })}
          disabled={createSpace.isPending}
        >
          <BookOpen className="mr-2 h-4 w-4" />
          {createSpace.isPending ? t(($) => $.wiki_page.creating) : t(($) => $.wiki_page.create_wiki)}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Top bar */}
      <div className="flex items-center gap-3 border-b px-4 py-2">
        <BookOpen className="h-5 w-5 text-muted-foreground" />
        <h1 className="text-sm font-semibold">{t(($) => $.wiki_page.title)}</h1>
        <div className="flex-1" />
        <Popover open={agentOpen} onOpenChange={setAgentOpen}>
          <PopoverTrigger>
            <Button variant="outline" size="sm" className="h-8 w-64 justify-between text-xs font-normal">
              {selectedAgent ? (
                <span className="flex items-center gap-1.5 truncate">
                  <ActorAvatar actorType="agent" actorId={selectedAgent.id} size={18} showStatusDot />
                  {selectedAgent.name}
                  <span className="text-muted-foreground">({selectedAgent.runtime_name || selectedAgent.runtime_provider || selectedAgent.runtime_mode || "cloud"})</span>
                </span>
              ) : <span className="text-muted-foreground">{t(($) => $.wiki_page.wiki_agent)}</span>}
              <ChevronDown className="ml-1 h-3.5 w-3.5 shrink-0 opacity-50" />
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-80 p-0" align="start">
            <div className="border-b px-3 py-2">
              <input
                className="w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                placeholder="Search agents..."
                value={agentSearch}
                onChange={(e) => setAgentSearch(e.target.value)}
              />
            </div>
            <div className="max-h-64 overflow-y-auto p-1">
              {filteredAgents.length === 0 ? (
                <p className="px-2 py-4 text-center text-xs text-muted-foreground">No agents found</p>
              ) : filteredAgents.map((a) => (
                <Tooltip key={a.id}>
                  <TooltipTrigger>
                    <button
                      className={`flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent ${a.id === wikiAgentId ? "bg-accent" : ""}`}
                      onClick={() => {
                        setWikiAgentId(a.id);
                        updateSpace.mutate({ slug: spaceSlug, data: { default_agent_id: a.id } });
                        setAgentOpen(false);
                        setAgentSearch("");
                      }}
                    >
                      <ActorAvatar actorType="agent" actorId={a.id} size={20} showStatusDot />
                      <span className="flex-1 truncate font-medium">{a.name}</span>
                      <span className="shrink-0 text-xs text-muted-foreground">({a.runtime_name || a.runtime_provider || a.runtime_mode || "cloud"})</span>
                      {a.id === wikiAgentId && <Check className="h-4 w-4 shrink-0" />}
                    </button>
                  </TooltipTrigger>
                  {a.description && <TooltipContent side="right" className="max-w-xs">{a.description}</TooltipContent>}
                </Tooltip>
              ))}
            </div>
          </PopoverContent>
        </Popover>
        <Button variant="outline" size="sm" onClick={() => { if (!wikiAgentId) { toast.error(t(($) => $.wiki_page.ingest_need_agent)); return; } setIngestOpen(true); }}>
          <Download className="mr-1 h-4 w-4" />
          {t(($) => $.wiki_page.ingest)}
        </Button>
        <div className="relative w-64">
          <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-8 pl-8 text-sm"
            placeholder={t(($) => $.wiki_page.search)}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
      </div>

      {/* Dual-panel body */}
      <div className="flex flex-1 overflow-hidden">
        <div className="w-64 flex-shrink-0 overflow-y-auto border-r">
          {pagesLoading ? (
            <div className="space-y-2 p-3">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-2/3" />
            </div>
          ) : (
            <WikiFileTree
              pages={pages ?? []}
              selectedPath={selectedPath}
              onSelect={handleSelectPage}
              selectedDir={selectedDir}
              onSelectDir={setSelectedDir}
              onCreateDir={handleCreateDir}
              onDelete={handleDelete}
              enabledDirs={enabledDirs}
            />
          )}
        </div>
        <div className="flex-1 overflow-y-auto">
          {!selectedPath ? (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              <div className="text-center">
                <FileText className="mx-auto h-10 w-10" />
                <p className="mt-2">{t(($) => $.wiki_page.select_page)}</p>
              </div>
            </div>
          ) : pageLoading ? (
            <div className="space-y-4 p-6">
              <Skeleton className="h-8 w-1/3" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-5/6" />
            </div>
          ) : pageDetail ? (
            <WikiPageViewer page={pageDetail} spaceSlug={spaceSlug} onSelect={handleSelectPage} editable={isPageEditable(pageDetail.path)} />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              Page not found
            </div>
          )}
        </div>
      </div>

      <WikiIngestDialog open={ingestOpen} onOpenChange={setIngestOpen} spaceSlug={spaceSlug} wikiAgentId={wikiAgentId} defaultDir={selectedDir || "raw"} />
    </div>
  );
}
