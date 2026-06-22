"use client";

import { Markdown } from "@multica/views/common/markdown";
import type { WikiPageDetail } from "@multica/core/wiki";

export function WikiPageViewer({ page }: { page: WikiPageDetail }) {
  return (
    <div className="mx-auto max-w-3xl px-6 py-8">
      <h1 className="mb-6 text-2xl font-bold tracking-tight">{page.title || page.path}</h1>
      <div className="mb-6 flex items-center gap-3 text-xs text-muted-foreground">
        {page.page_type && <span className="rounded bg-accent px-2 py-0.5">{page.page_type}</span>}
        <span>Updated: {new Date(page.updated_at).toLocaleDateString()}</span>
      </div>
      <div className="prose prose-sm max-w-none dark:prose-invert">
        <Markdown>{page.content}</Markdown>
      </div>
      {page.links && page.links.length > 0 && (
        <div className="mt-8 border-t pt-6">
          <h3 className="mb-2 text-sm font-semibold">Links</h3>
          <ul className="space-y-1">
            {page.links.map((link: { target: string; title?: string | null; snippet?: string | null; exists: boolean }) => (
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
      {page.backlinks && page.backlinks.length > 0 && (
        <div className="mt-6 border-t pt-6">
          <h3 className="mb-2 text-sm font-semibold">Backlinks ({page.backlinks.length})</h3>
          <ul className="space-y-2">
            {page.backlinks.map((bl: { source: string; title?: string | null; context?: string | null }) => (
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
