"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Globe, FileUp, PenLine, Inbox, Loader2 } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { inboxListOptions } from "@multica/core/inbox";
import { useCreateWikiSource } from "@multica/core/wiki";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Skeleton } from "@multica/ui/components/ui/skeleton";

interface Props { open: boolean; onOpenChange: (v: boolean) => void; spaceSlug: string; }

export function WikiIngestDialog({ open, onOpenChange, spaceSlug }: Props) {
  const wsId = useWorkspaceId();
  const createSource = useCreateWikiSource(wsId, spaceSlug);
  const [tab, setTab] = useState("inbox");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [url, setUrl] = useState("");
  const [urlPreview, setUrlPreview] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [md, setMd] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const { data: inboxItems, isLoading: inboxLoading } = useQuery(inboxListOptions(wsId));

  const done = () => { setBusy(false); setError(""); onOpenChange(false); };
  const fail = (msg: string) => { setError(msg); setBusy(false); };

  const handleInbox = () => {
    if (selected.size === 0) return;
    setBusy(true); setError("");
    const ids = Array.from(selected);
    let remaining = ids.length;
    ids.forEach((id) => {
      const item = inboxItems?.find((i) => i.id === id);
      createSource.mutate(
        { title: item?.title || `Inbox ${id}`, content: `# ${item?.title || "Inbox item"}\n\n${item?.body || ""}`, source_type: "inbox", raw_path: `raw/inbox-${Date.now()}.md` },
        { onSuccess: () => { if (--remaining === 0) { setSelected(new Set()); done(); } }, onError: (e: any) => fail(e?.message || "Import failed") },
      );
    });
  };

  const handleCrawl = async () => {
    if (!url) return;
    setBusy(true); setError("");
    try {
      const resp = await fetch(url.startsWith("http") ? url : `https://${url}`);
      const text = await resp.text();
      setUrlPreview(text.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim().slice(0, 5000));
    } catch (e: any) { setUrlPreview(`Crawl failed: ${e?.message || "unknown error"}`); }
    setBusy(false);
  };

  const handleUrl = () => {
    if (!url) return;
    setBusy(true); setError("");
    createSource.mutate(
      { title: url, content: urlPreview || `# URL Source\n\n${url}`, url, source_type: "url", raw_path: `raw/url-${Date.now()}.md` },
      { onSuccess: () => { setUrl(""); setUrlPreview(""); done(); }, onError: (e: any) => fail(e?.message || "Failed") },
    );
  };

  const handleFile = () => {
    if (!file) return;
    setBusy(true); setError("");
    const isText = file.name.endsWith(".md") || file.name.endsWith(".txt");
    const reader = new FileReader();
    reader.onload = () => {
      const content = isText ? (reader.result as string) : `# ${file.name}\n\n> Binary file.\n> Size: ${(file.size / 1024).toFixed(1)} KB`;
      createSource.mutate(
        { title: file.name, content, source_type: "file", raw_path: `raw/${file.name}` },
        { onSuccess: () => { setFile(null); done(); }, onError: (e: any) => fail(e?.message || "Upload failed") },
      );
    };
    reader.onerror = () => fail("Failed to read file");
    if (isText) reader.readAsText(file); else reader.readAsDataURL(file);
  };

  const handleMd = () => {
    if (!md.trim()) return;
    setBusy(true); setError("");
    const firstLine = md.trim().split("\n")[0]?.replace(/^#\s*/, "") || "Manual entry";
    createSource.mutate(
      { title: firstLine, content: md, source_type: "manual", raw_path: `raw/manual-${Date.now()}.md` },
      { onSuccess: () => { setMd(""); done(); }, onError: (e: any) => fail(e?.message || "Save failed") },
    );
  };

  const toggle = (id: string) => { const n = new Set(selected); n.has(id) ? n.delete(id) : n.add(id); setSelected(n); };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="!max-w-[60vw] flex h-[80vh] flex-col overflow-hidden">
        <DialogHeader className="shrink-0"><DialogTitle>Ingest Knowledge</DialogTitle></DialogHeader>
        <Tabs value={tab} onValueChange={setTab} className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <TabsList className="grid w-full shrink-0 grid-cols-4">
            <TabsTrigger value="inbox"><Inbox className="mr-1 h-3.5 w-3.5" />Inbox</TabsTrigger>
            <TabsTrigger value="url"><Globe className="mr-1 h-3.5 w-3.5" />URL</TabsTrigger>
            <TabsTrigger value="file"><FileUp className="mr-1 h-3.5 w-3.5" />File</TabsTrigger>
            <TabsTrigger value="markdown"><PenLine className="mr-1 h-3.5 w-3.5" />Write</TabsTrigger>
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
            <Button onClick={handleInbox} disabled={selected.size === 0 || busy} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest{selected.size > 0 ? ` (${selected.size})` : ""}
            </Button>
          </TabsContent>

          {/* URL */}
          <TabsContent value="url" className="flex min-h-0 flex-1 flex-col space-y-2 pt-2 data-[state=inactive]:hidden">
            <div className="flex shrink-0 gap-2">
              <Input placeholder="https://example.com/article" value={url} onChange={(e) => setUrl(e.target.value)} className="flex-1" />
              <Button variant="outline" onClick={handleCrawl} disabled={!url || busy}>
                {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Globe className="mr-1 h-4 w-4" />}Crawl
              </Button>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto rounded border bg-muted/50 p-3">
              {urlPreview ? (
                <pre className="whitespace-pre-wrap text-xs text-muted-foreground">{urlPreview}</pre>
              ) : (
                <p className="text-xs text-muted-foreground">Enter a URL and click Crawl to preview content.</p>
              )}
            </div>
            {error && <p className="shrink-0 text-xs text-destructive">{error}</p>}
            <Button onClick={handleUrl} disabled={!url || busy} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest
            </Button>
          </TabsContent>

          {/* File */}
          <TabsContent value="file" className="flex min-h-0 flex-1 flex-col space-y-2 pt-2 data-[state=inactive]:hidden">
            <Input type="file" accept=".md,.txt,.pdf,.xls,.xlsx,.doc,.docx,.wps" onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="shrink-0" />
            {file && <p className="shrink-0 text-xs text-muted-foreground">{file.name} ({(file.size / 1024).toFixed(1)} KB)</p>}
            <div className="flex-1" />
            {error && <p className="shrink-0 text-xs text-destructive">{error}</p>}
            <Button onClick={handleFile} disabled={!file || busy} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest
            </Button>
          </TabsContent>

          {/* Markdown */}
          <TabsContent value="markdown" className="flex min-h-0 flex-1 flex-col space-y-2 pt-2 data-[state=inactive]:hidden">
            <Textarea className="min-h-0 flex-1 resize-none font-mono text-sm" placeholder="# Title\n\nWrite here..." value={md} onChange={(e) => setMd(e.target.value)} />
            {error && <p className="shrink-0 text-xs text-destructive">{error}</p>}
            <Button onClick={handleMd} disabled={!md.trim() || busy} className="shrink-0">
              {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest
            </Button>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
