"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Globe, FileUp, PenLine, Inbox, Loader2 } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { inboxListOptions } from "@multica/core/inbox";
import { useCreateWikiSource, useCreateWikiOperation } from "@multica/core/wiki";
import { useT } from "../../i18n";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Skeleton } from "@multica/ui/components/ui/skeleton";

interface Props { open: boolean; onOpenChange: (v: boolean) => void; spaceSlug: string; wikiAgentId?: string; defaultDir?: string; }

export function WikiIngestDialog({ open, onOpenChange, spaceSlug, wikiAgentId, defaultDir }: Props) {
  const wsId = useWorkspaceId();
  const { t } = useT("layout");
  const createSource = useCreateWikiSource(wsId, spaceSlug);
  const createOp = useCreateWikiOperation(wsId, spaceSlug);
  const [tab, setTab] = useState("inbox");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [url, setUrl] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [md, setMd] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  const targetDir = (defaultDir && defaultDir.startsWith("raw")) ? defaultDir : "raw";

  const { data: inboxItems, isLoading: inboxLoading } = useQuery(inboxListOptions(wsId));

  const done = () => {
    setBusy(""); setError(""); onOpenChange(false);
    // Trigger wiki maintainer agent to organize ingested content
    if (wikiAgentId) {
      createOp.mutate({ operation_type: "ingest", title: "Process new raw sources", prompt: "Review raw/ for new sources and ingest them into wiki/." });
    }
  };
  const fail = (msg: string) => { setError(msg); setBusy(""); };

  const handleInbox = () => {
    if (selected.size === 0) return;
    setBusy("inbox"); setError("");
    const ids = Array.from(selected);
    let remaining = ids.length;
    ids.forEach((id) => {
      const item = inboxItems?.find((i) => i.id === id);
      createSource.mutate(
        { title: item?.title || `Inbox ${id}`, content: `# ${item?.title || "Inbox item"}\n\n${item?.body || ""}`, source_type: "inbox", raw_path: `${targetDir}/inbox-${Date.now()}.md` },
        { onSuccess: () => { if (--remaining === 0) { setSelected(new Set()); done(); } }, onError: (e: any) => fail(e?.message || "Import failed") },
      );
    });
  };

  const handleUrl = () => {
    if (!url) return;
    setBusy("url"); setError("");
    createSource.mutate(
      { title: url, content: `# URL Source\n\n${url}`, url, source_type: "url", raw_path: `${targetDir}/url-${Date.now()}.md` },
      { onSuccess: () => { setUrl(""); done(); }, onError: (e: any) => fail(e?.message || "Failed") },
    );
  };

  const handleFile = () => {
    if (!file) return;
    setBusy("file"); setError("");
    const isText = file.name.endsWith(".md") || file.name.endsWith(".txt");
    const reader = new FileReader();
    reader.onload = () => {
      const content = isText ? (reader.result as string) : `# ${file.name}\n\n> Binary file.\n> Size: ${(file.size / 1024).toFixed(1)} KB`;
      createSource.mutate(
        { title: file.name, content, source_type: "file", raw_path: `${targetDir}/${file.name}` },
        { onSuccess: () => { setFile(null); done(); }, onError: (e: any) => fail(e?.message || "Upload failed") },
      );
    };
    reader.onerror = () => fail("Failed to read file");
    if (isText) reader.readAsText(file); else reader.readAsDataURL(file);
  };

  const handleMd = () => {
    if (!md.trim()) return;
    setBusy("markdown"); setError("");
    const firstLine = md.trim().split("\n")[0]?.replace(/^#\s*/, "") || "Manual entry";
    createSource.mutate(
      { title: firstLine, content: md, source_type: "manual", raw_path: `${targetDir}/manual-${Date.now()}.md` },
      { onSuccess: () => { setMd(""); done(); }, onError: (e: any) => fail(e?.message || "Save failed") },
    );
  };

  const toggle = (id: string) => { const n = new Set(selected); n.has(id) ? n.delete(id) : n.add(id); setSelected(n); };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="!max-w-[60vw] flex h-[80vh] flex-col overflow-hidden">
        <DialogHeader className="shrink-0"><DialogTitle>{t(($) => $.wiki_page.ingest_title)}</DialogTitle></DialogHeader>
        <Tabs value={tab} onValueChange={setTab} className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <TabsList className="grid w-full shrink-0 grid-cols-4">
            <TabsTrigger value="inbox"><Inbox className="mr-1 h-3.5 w-3.5" />{t(($) => $.wiki_page.ingest_inbox)}</TabsTrigger>
            <TabsTrigger value="url"><Globe className="mr-1 h-3.5 w-3.5" />{t(($) => $.wiki_page.ingest_url)}</TabsTrigger>
            <TabsTrigger value="file"><FileUp className="mr-1 h-3.5 w-3.5" />{t(($) => $.wiki_page.ingest_file)}</TabsTrigger>
            <TabsTrigger value="markdown"><PenLine className="mr-1 h-3.5 w-3.5" />{t(($) => $.wiki_page.ingest_write)}</TabsTrigger>
          </TabsList>

          {/* Inbox */}
          <TabsContent value="inbox" className="flex min-h-0 flex-1 flex-col space-y-2 pt-2 data-[state=inactive]:hidden">
            {inboxLoading ? <Skeleton className="h-full w-full" /> : (inboxItems?.length ?? 0) === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">No inbox items</p>
            ) : (
              <div className="min-h-0 flex-1 space-y-1 overflow-y-auto">
                {(inboxItems || []).slice(0, 50).map((item) => (
                  <label key={item.id} className={`flex cursor-pointer gap-2 rounded border p-2 text-sm hover:bg-accent ${selected.has(item.id) ? "border-primary bg-accent" : ""}`}>
                    <input type="checkbox" className="mt-1" checked={selected.has(item.id)} onChange={() => toggle(item.id)} />
                    <div className="min-w-0"><p className="truncate font-medium">{item.title || "(no subject)"}</p><p className="truncate text-xs text-muted-foreground">{item.body?.slice(0, 100)}</p></div>
                  </label>
                ))}
              </div>
            )}
            {error && <p className="shrink-0 text-xs text-destructive">{error}</p>}
            <Button onClick={handleInbox} disabled={selected.size === 0 || busy === "inbox"} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}{t(($) => $.wiki_page.ingest)}{selected.size > 0 ? ` (${selected.size})` : ""}
            </Button>
          </TabsContent>

          {/* URL */}
          <TabsContent value="url" className="flex min-h-0 flex-1 flex-col space-y-2 pt-2 data-[state=inactive]:hidden">
            <Input placeholder="https://example.com/article" value={url} onChange={(e) => setUrl(e.target.value)} className="shrink-0" />
            <p className="text-xs text-muted-foreground">Enter a URL to save as a knowledge source. The AI Agent will crawl and ingest it later.</p>
            <div className="flex-1" />
            {error && <p className="shrink-0 text-xs text-destructive">{error}</p>}
            <Button onClick={handleUrl} disabled={!url || busy === "url"} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}{t(($) => $.wiki_page.ingest)}
            </Button>
          </TabsContent>

          {/* File */}
          <TabsContent value="file" className="flex min-h-0 flex-1 flex-col space-y-2 pt-2 data-[state=inactive]:hidden">
            <Input type="file" accept=".md,.txt,.pdf,.xls,.xlsx,.doc,.docx,.wps" onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="shrink-0" />
            {file && <p className="shrink-0 text-xs text-muted-foreground">{file.name} ({(file.size / 1024).toFixed(1)} KB)</p>}
            <div className="flex-1" />
            {error && <p className="shrink-0 text-xs text-destructive">{error}</p>}
            <Button onClick={handleFile} disabled={!file || busy === "file"} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}{t(($) => $.wiki_page.ingest)}
            </Button>
          </TabsContent>

          {/* Markdown */}
          <TabsContent value="markdown" className="flex min-h-0 flex-1 flex-col space-y-2 pt-2 data-[state=inactive]:hidden">
            <Textarea className="min-h-0 flex-1 resize-none font-mono text-sm" placeholder="# Title\n\nWrite here..." value={md} onChange={(e) => setMd(e.target.value)} />
            {error && <p className="shrink-0 text-xs text-destructive">{error}</p>}
            <Button onClick={handleMd} disabled={!md.trim() || busy === "markdown"} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}{t(($) => $.wiki_page.ingest)}
            </Button>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
