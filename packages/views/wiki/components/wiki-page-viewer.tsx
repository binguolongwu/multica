"use client";

import { useState } from "react";
import { Pencil, Check, X } from "lucide-react";
import { Markdown } from "@multica/views/common/markdown";
import { useUpsertWikiPage } from "@multica/core/wiki";
import { useWorkspaceId } from "@multica/core/hooks";
import type { WikiPageDetail } from "@multica/core/wiki";
import { Button } from "@multica/ui/components/ui/button";

interface WikiPageViewerProps {
  page: WikiPageDetail;
  spaceSlug?: string;
}

export function WikiPageViewer({ page, spaceSlug = "default" }: WikiPageViewerProps) {
  const wsId = useWorkspaceId();
  const [editing, setEditing] = useState(false);
  const [editContent, setEditContent] = useState(page.content);
  const upsert = useUpsertWikiPage(wsId, spaceSlug);

  const handleSave = () => {
    upsert.mutate(
      { path: page.path, data: { content: editContent, summary: "Manual edit" } },
      { onSuccess: () => setEditing(false) },
    );
  };

  const handleCancel = () => {
    setEditContent(page.content);
    setEditing(false);
  };

  return (
    <div className="mx-auto max-w-3xl px-6 py-8">
      {/* Header with edit button */}
      <div className="mb-6 flex items-start justify-between gap-4">
        <h1 className="text-2xl font-bold tracking-tight">{page.title || page.path}</h1>
        {!editing && (
          <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
            <Pencil className="mr-1 h-3.5 w-3.5" />
            Edit
          </Button>
        )}
      </div>

      {/* Meta bar */}
      <div className="mb-6 flex items-center gap-3 text-xs text-muted-foreground">
        {page.page_type && <span className="rounded bg-accent px-2 py-0.5">{page.page_type}</span>}
        <span>Updated: {new Date(page.updated_at).toLocaleDateString()}</span>
      </div>

      {/* Content: edit or view */}
      {editing ? (
        <div className="space-y-4">
          <textarea
            className="min-h-[400px] w-full rounded-md border bg-background p-4 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
          />
          <div className="flex gap-2">
            <Button size="sm" onClick={handleSave} disabled={upsert.isPending}>
              <Check className="mr-1 h-3.5 w-3.5" />
              {upsert.isPending ? "Saving..." : "Save"}
            </Button>
            <Button size="sm" variant="outline" onClick={handleCancel}>
              <X className="mr-1 h-3.5 w-3.5" />
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <div className="prose prose-sm max-w-none dark:prose-invert">
          <Markdown>{page.content}</Markdown>
        </div>
      )}

      {/* Links */}
      {page.links && page.links.length > 0 && (
        <div className="mt-8 border-t pt-6">
          <h3 className="mb-2 text-sm font-semibold">Links</h3>
          <ul className="space-y-1">
            {page.links.map((link) => (
              <li key={link.target} className="text-sm">
                <span className={link.exists ? "text-primary" : "text-muted-foreground line-through"}>
                  {link.title || link.target}
                </span>
                {link.snippet && <span className="ml-2 text-xs text-muted-foreground">— {link.snippet}</span>}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Backlinks */}
      {page.backlinks && page.backlinks.length > 0 && (
        <div className="mt-6 border-t pt-6">
          <h3 className="mb-2 text-sm font-semibold">Backlinks ({page.backlinks.length})</h3>
          <ul className="space-y-2">
            {page.backlinks.map((bl) => (
              <li key={bl.source} className="text-sm">
                <span className="font-medium text-primary">{bl.title || bl.source}</span>
                {bl.context && <p className="mt-0.5 text-xs text-muted-foreground">{bl.context}</p>}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
