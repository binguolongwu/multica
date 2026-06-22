"use client";

import { useState, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileText, BookOpen, Search, Loader2 } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { wikiSpacesOptions, wikiPagesOptions, wikiPageDetailOptions, useCreateWikiSpace } from "@multica/core/wiki";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { WikiFileTree } from "./wiki-file-tree";
import { WikiPageViewer } from "./wiki-page-viewer";

const DEFAULT_SPACE = "default";

export function WikiPage() {
  const wsId = useWorkspaceId();
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [spaceSlug] = useState(DEFAULT_SPACE);

  const { data: spaces, isLoading: spacesLoading } = useQuery(wikiSpacesOptions(wsId));
  const createSpace = useCreateWikiSpace(wsId);
  const { data: pages, isLoading: pagesLoading } = useQuery(
    wikiPagesOptions(wsId, spaceSlug, searchQuery ? { search: searchQuery } : undefined),
  );
  const { data: pageDetail, isLoading: pageLoading } = useQuery(
    wikiPageDetailOptions(wsId, spaceSlug, selectedPath ?? ""),
  );

  const handleSelectPage = useCallback((path: string) => {
    setSelectedPath(path);
    setSearchQuery("");
  }, []);

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
          <h2 className="text-lg font-semibold">Wiki not set up</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Create a wiki space to start building your knowledge base.
          </p>
        </div>
        <Button
          onClick={() => createSpace.mutate({ display_name: "default" })}
          disabled={createSpace.isPending}
        >
          <BookOpen className="mr-2 h-4 w-4" />
          {createSpace.isPending ? "Creating..." : "Create Wiki"}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Top bar */}
      <div className="flex items-center gap-3 border-b px-4 py-2">
        <BookOpen className="h-5 w-5 text-muted-foreground" />
        <h1 className="text-sm font-semibold">Wiki</h1>
        <div className="flex-1" />
        <div className="relative w-64">
          <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-8 pl-8 text-sm"
            placeholder="Search wiki..."
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
            <WikiFileTree pages={pages ?? []} selectedPath={selectedPath} onSelect={handleSelectPage} />
          )}
        </div>
        <div className="flex-1 overflow-y-auto">
          {!selectedPath ? (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              <div className="text-center">
                <FileText className="mx-auto h-10 w-10" />
                <p className="mt-2">Select a page from the tree to read</p>
              </div>
            </div>
          ) : pageLoading ? (
            <div className="space-y-4 p-6">
              <Skeleton className="h-8 w-1/3" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-5/6" />
            </div>
          ) : pageDetail ? (
            <WikiPageViewer page={pageDetail} />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              Page not found
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
