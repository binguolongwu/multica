"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Download, Globe, FileUp, PenLine, Inbox, Loader2 } from "lucide-react";
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

  const clear = () => { setBusy(false); setError(""); onOpenChange(false); };

  const handleInbox = async () => {
    if (selected.size === 0) return;
    setBusy(true); setError("");
    try {
      for (const id of selected) {
        const item = inboxItems?.find((i) => i.id === id);
        await createSource.mutateAsync({
          title: item?.title || `Inbox ${id}`,
          content: `# ${item?.title || "Inbox item"}\n\n${item?.body || ""}`,
          source_type: "inbox",
          raw_path: `raw/inbox-${Date.now()}.md`,
        });
      }
      setSelected(new Set()); clear();
    } catch (e: any) { setError(e?.message || "Import failed"); setBusy(false); }
  };

  const handleCrawl = async () => {
    if (!url) return;
    setBusy(true); setError("");
    try {
      const resp = await fetch(url.startsWith("http") ? url : `https://${url}`);
      const text = await resp.text();
      // Strip HTML tags for basic preview
      const stripped = text.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim().slice(0, 3000);
      setUrlPreview(`# URL Source: ${url}\n\n${stripped}...`);
    } catch (e: any) {
      setUrlPreview(`# URL Source: ${url}\n\n> Crawl failed: ${e?.message || "unknown error"}\n> URL saved as reference.`);
    }
    setBusy(false);
  };

  const handleUrl = async () => {
    if (!url) return;
    setBusy(true); setError("");
    try {
      await createSource.mutateAsync({
        title: url,
        content: urlPreview || `# URL Source\n\n${url}\n\n> Pending crawl.`,
        url,
        source_type: "url",
        raw_path: `raw/url-${Date.now()}.md`,
      });
      setUrl(""); setUrlPreview(""); clear();
    } catch (e: any) { setError(e?.message || "Failed"); setBusy(false); }
  };

  const handleFile = async () => {
    if (!file) return;
    setBusy(true); setError("");
    try {
      let content = "";
      if (file.name.endsWith(".md") || file.name.endsWith(".txt")) {
        content = await file.text();
      } else {
        content = `# ${file.name}\n\n> Binary file uploaded as reference.\n> Type: ${file.type || "unknown"}\n> Size: ${(file.size / 1024).toFixed(1)} KB`;
      }
      await createSource.mutateAsync({
        title: file.name,
        content,
        source_type: "file",
        raw_path: `raw/${file.name}`,
      });
      setFile(null); clear();
    } catch (e: any) { setError(e?.message || "Upload failed"); setBusy(false); }
  };

  const handleMd = async () => {
    if (!md.trim()) return;
    setBusy(true); setError("");
    try {
      const firstLine = md.trim().split("\n")[0]?.replace(/^#\s*/, "") || "Manual entry";
      await createSource.mutateAsync({
        title: firstLine,
        content: md,
        source_type: "manual",
        raw_path: `raw/manual-${Date.now()}.md`,
      });
      setMd(""); clear();
    } catch (e: any) { setError(e?.message || "Save failed"); setBusy(false); }
  };

  const toggle = (id: string) => {
    const n = new Set(selected); n.has(id) ? n.delete(id) : n.add(id); setSelected(n);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="!max-w-[60vw] flex max-h-[80vh] flex-col overflow-hidden">
        <DialogHeader className="shrink-0"><DialogTitle>Ingest Knowledge</DialogTitle></DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto">
          <Tabs value={tab} onValueChange={setTab}>
            <TabsList className="grid w-full grid-cols-4">
              <TabsTrigger value="inbox"><Inbox className="mr-1 h-3.5 w-3.5" />Inbox</TabsTrigger>
              <TabsTrigger value="url"><Globe className="mr-1 h-3.5 w-3.5" />URL</TabsTrigger>
              <TabsTrigger value="file"><FileUp className="mr-1 h-3.5 w-3.5" />File</TabsTrigger>
              <TabsTrigger value="markdown"><PenLine className="mr-1 h-3.5 w-3.5" />Write</TabsTrigger>
            </TabsList>

            {/* Inbox */}
            <TabsContent value="inbox" className="space-y-3 pt-3">
              {inboxLoading ? <Skeleton className="h-12 w-full" /> : (inboxItems?.length ?? 0) === 0 ? (
                <p className="py-8 text-center text-sm text-muted-foreground">No inbox items</p>
              ) : (
                <div className="max-h-40 space-y-1 overflow-y-auto">
                  {(inboxItems || []).slice(0, 30).map((item) => (
                    <label key={item.id} className={`flex cursor-pointer gap-2 rounded border p-2 text-sm hover:bg-accent ${selected.has(item.id) ? "border-primary bg-accent" : ""}`}>
                      <input type="checkbox" className="mt-1" checked={selected.has(item.id)} onChange={() => toggle(item.id)} />
                      <div className="min-w-0"><p className="truncate font-medium">{item.title || "(no subject)"}</p><p className="truncate text-xs text-muted-foreground">{item.body?.slice(0, 100)}</p></div>
                    </label>
                  ))}
                </div>
              )}
              {error && <p className="text-xs text-destructive">{error}</p>}
              <Button onClick={handleInbox} disabled={selected.size === 0 || busy} className="w-full">
                {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest{selected.size > 0 ? ` (${selected.size})` : ""}
              </Button>
            </TabsContent>

            {/* URL */}
            <TabsContent value="url" className="space-y-3 pt-3">
              <div className="flex gap-2">
                <Input placeholder="https://example.com/article" value={url} onChange={(e) => setUrl(e.target.value)} className="flex-1" />
                <Button variant="outline" onClick={handleCrawl} disabled={!url || busy}>
                  {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Globe className="mr-1 h-4 w-4" />}Crawl
                </Button>
              </div>
              {urlPreview && (
                <div className="max-h-40 overflow-y-auto rounded border bg-muted/50 p-3">
                  <pre className="whitespace-pre-wrap text-xs text-muted-foreground">{urlPreview}</pre>
                </div>
              )}
              {error && <p className="text-xs text-destructive">{error}</p>}
              <Button onClick={handleUrl} disabled={!url || busy} className="w-full">
                {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest
              </Button>
            </TabsContent>

            {/* File */}
            <TabsContent value="file" className="space-y-3 pt-3">
              <Input type="file" accept=".md,.txt,.pdf,.xls,.xlsx,.doc,.docx,.wps" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
              {file && <p className="text-xs text-muted-foreground">{file.name} ({(file.size / 1024).toFixed(1)} KB)</p>}
              {error && <p className="text-xs text-destructive">{error}</p>}
              <Button onClick={handleFile} disabled={!file || busy} className="w-full">
                {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest
              </Button>
            </TabsContent>

            {/* Markdown */}
            <TabsContent value="markdown" className="space-y-3 pt-3">
              <Textarea className="min-h-[200px] font-mono text-sm" placeholder="# Title\n\nWrite here..." value={md} onChange={(e) => setMd(e.target.value)} />
              {error && <p className="text-xs text-destructive">{error}</p>}
              <Button onClick={handleMd} disabled={!md.trim() || busy} className="w-full">
                {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}Ingest
              </Button>
            </TabsContent>
          </Tabs>
        </div>
      </DialogContent>
    </Dialog>
  );
}
