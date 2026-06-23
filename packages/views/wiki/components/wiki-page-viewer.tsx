"use client";

import { useState, useMemo } from "react";
import { Pencil, Check, X } from "lucide-react";
import { Markdown } from "@multica/views/common/markdown";
import { useUpsertWikiPage } from "@multica/core/wiki";
import { useWorkspaceId } from "@multica/core/hooks";
import type { WikiPageDetail } from "@multica/core/wiki";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

function parseFrontmatter(content: string): { body: string; fm: Record<string, string> } {
  if (!content.startsWith("---\n")) return { body: content, fm: {} };
  const end = content.indexOf("\n---", 4);
  if (end === -1) return { body: content, fm: {} };
  const fmBlock = content.slice(4, end);
  const body = content.slice(end + 4);
  const fm: Record<string, string> = {};
  for (const line of fmBlock.split("\n")) {
    const m = line.match(/^([A-Za-z_-]+):\s*(.*)/);
    const key = m?.[1]; const val = m?.[2];
    if (key && val != null) fm[key] = val.replace(/^["']|["']$/g, "");
  }
  return { body, fm };
}

function extractHeadings(content: string): { id: string; text: string; level: number }[] {
  const headings: { id: string; text: string; level: number }[] = [];
  const re = /^(#{1,4})\s+(.+)$/gm;
  let m;
  while ((m = re.exec(content))) {
    const hText = m[2]?.trim();
    const hLevel = m[1]?.length ?? 1;
    if (!hText) continue;
    const id = hText.toLowerCase().replace(/[^a-z0-9一-鿿]+/g, "-").replace(/^-|-$/g, "");
    headings.push({ id, text: hText, level: hLevel });
  }
  return headings;
}

function resolveWikilinks(content: string): string {
  return content.replace(/\[\[([^\]]+)\]\]/g, (_m, path: string) => {
    const display = path.split("/").pop() || path;
    return `<a class="wiki-link" data-wiki-path="${path}" href="#">${display}</a>`;
  });
}

export function WikiPageViewer({ page, spaceSlug = "default", onSelect }: { page: WikiPageDetail; spaceSlug?: string; onSelect?: (path: string) => void }) {
  const wsId = useWorkspaceId();
  const { t } = useT("layout");
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editContent, setEditContent] = useState("");
  const upsert = useUpsertWikiPage(wsId, spaceSlug);

  const { body, fm } = useMemo(() => parseFrontmatter(page.content), [page.content]);
  const headings = useMemo(() => extractHeadings(body), [body]);
  // Escape HTML in content before rendering to prevent tags from being
  // interpreted as raw HTML by the markdown renderer's rehype-raw pass.
  // Then resolve [[wikilinks]] to clickable <a> tags.
  const safeBody = resolveWikilinks(body.replace(/</g, "&lt;").replace(/>/g, "&gt;"));
  const displayTitle = fm.title || page.title || page.path;

  const handleSave = () => {
    // Rebuild content with frontmatter
    const fmLines: string[] = ["---", `title: ${editTitle}`];
    Object.entries(fm).filter(([k]) => k !== "title").forEach(([k, v]) => fmLines.push(`${k}: ${v}`));
    fmLines.push("---");
    const fullContent = fmLines.join("\n") + "\n\n" + editContent;
    upsert.mutate(
      { path: page.path, data: { content: fullContent, summary: "Manual edit" } },
      { onSuccess: () => setEditing(false) },
    );
  };

  return (
    <div className="flex h-full">
      <div className="min-w-0 flex-1 overflow-y-auto px-6 py-8">
        <div className="mx-auto max-w-3xl">
          <div className="mb-6 flex items-start justify-between gap-4">
            <h1 className="text-2xl font-bold tracking-tight">{displayTitle}</h1>
            {!editing && (
              <Button variant="outline" size="sm" onClick={() => { setEditTitle(displayTitle); setEditContent(body); setEditing(true); }}>
                <Pencil className="mr-1 h-3.5 w-3.5" />{t(($) => $.wiki_page.edit)}
              </Button>
            )}
          </div>
          <div className="mb-6 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            {page.page_type && <span className="rounded bg-accent px-2 py-0.5">{page.page_type}</span>}
            {fm.tags && fm.tags.split(",").map((t: string) => (
              <span key={t} className="rounded bg-muted px-1.5 py-0.5">{t.trim()}</span>
            ))}
            <span>Updated: {new Date(page.updated_at).toLocaleDateString('zh-CN')}</span>
          </div>
          {Object.keys(fm).filter((k) => k !== "title" && k !== "tags").length > 0 && (
            <details className="mb-6 rounded border bg-muted/30 px-4 py-2">
              <summary className="cursor-pointer text-xs font-medium text-muted-foreground">{t(($) => $.wiki_page.properties)}</summary>
              <dl className="mt-2 space-y-1 text-xs">
                {Object.entries(fm).filter(([k]) => k !== "title" && k !== "tags").map(([k, v]) => (
                  <div key={k} className="flex gap-2"><dt className="font-medium text-muted-foreground">{k}:</dt><dd>{v}</dd></div>
                ))}
              </dl>
            </details>
          )}
          {editing ? (
            <div className="space-y-4">
              <input
                className="w-full rounded-md border bg-background px-3 py-2 text-lg font-semibold focus:outline-none focus:ring-2 focus:ring-primary"
                value={editTitle}
                onChange={(e) => setEditTitle(e.target.value)}
                placeholder="Page title"
              />
              <textarea className="min-h-[400px] w-full resize-y rounded-md border bg-background p-4 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-primary" value={editContent} onChange={(e) => setEditContent(e.target.value)} />
              <div className="flex gap-2">
                <Button size="sm" onClick={handleSave} disabled={upsert.isPending}>{upsert.isPending ? t(($) => $.wiki_page.saving) : <><Check className="mr-1 h-3.5 w-3.5" />{t(($) => $.wiki_page.save)}</>}</Button>
                <Button size="sm" variant="outline" onClick={() => { setEditTitle(displayTitle); setEditContent(body); setEditing(false); }}><X className="mr-1 h-3.5 w-3.5" />{t(($) => $.wiki_page.cancel)}</Button>
              </div>
            </div>
          ) : (
            <div
            className="prose prose-sm max-w-none dark:prose-invert"
            onClick={(e) => {
              const anchor = (e.target as HTMLElement).closest("a.wiki-link");
              if (!anchor) return;
              e.preventDefault();
              const path = anchor.getAttribute("data-wiki-path");
              if (path && onSelect) onSelect(path);
            }}
          >
              <Markdown>{safeBody}</Markdown>
            </div>
          )}
          {page.links && page.links.length > 0 && (
            <div className="mt-8 border-t pt-6">
              <h3 className="mb-2 text-sm font-semibold">{t(($) => $.wiki_page.links)}</h3>
              <ul className="space-y-1">{page.links.map((l) => <li key={l.target} className="text-sm"><span className={l.exists ? "text-primary" : "text-muted-foreground line-through"}>{l.title || l.target}</span>{l.snippet && <span className="ml-2 text-xs text-muted-foreground">— {l.snippet}</span>}</li>)}</ul>
            </div>
          )}
          {page.backlinks && page.backlinks.length > 0 && (
            <div className="mt-6 border-t pt-6">
              <h3 className="mb-2 text-sm font-semibold">{t(($) => $.wiki_page.backlinks)} ({page.backlinks.length})</h3>
              <ul className="space-y-2">{page.backlinks.map((bl) => <li key={bl.source} className="text-sm"><span className="font-medium text-primary">{bl.title || bl.source}</span>{bl.context && <p className="mt-0.5 text-xs text-muted-foreground">{bl.context}</p>}</li>)}</ul>
            </div>
          )}
        </div>
      </div>
      {headings.length > 1 && (
        <aside className="hidden w-44 shrink-0 overflow-y-auto border-l px-3 py-8 xl:block">
          <h4 className="mb-2 text-xs font-semibold text-muted-foreground">{t(($) => $.wiki_page.on_this_page)}</h4>
          <nav className="space-y-0.5">
            {headings.map((h) => (
              <button
                key={h.id}
                className="block w-full truncate rounded px-2 py-0.5 text-left text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
                style={{ paddingLeft: 8 + (h.level - 1) * 12 }}
                onClick={() => {
                  const el = document.getElementById(h.id) || [...document.querySelectorAll('h1,h2,h3,h4')].find(e => e.textContent?.trim() === h.text);
                  el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                }}
              >
                {h.text}
              </button>
            ))}
          </nav>
        </aside>
      )}
    </div>
  );
}
