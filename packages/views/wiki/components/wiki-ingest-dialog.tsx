"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Download, Globe, FileUp, PenLine, Inbox } from "lucide-react";
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
  const [file, setFile] = useState<File | null>(null);
  const [md, setMd] = useState("");
  const [busy, setBusy] = useState(false);

  const { data: inboxItems, isLoading: inboxLoading } = useQuery(inboxListOptions(wsId));

  const done = () => { setBusy(false); onOpenChange(false); };

  const handleInbox = async () => {
    if (selected.size === 0) return;
    setBusy(true);
    for (const id of selected) {
      const item = inboxItems?.find((i) => i.id === id);
      const title = item?.title || `Inbox ${id}`;
      const content = `# ${title}\n\n${item?.body || ""}`;
      await createSource.mutateAsync({ title, content, source_type: "inbox", raw_path: `raw/inbox-${Date.now()}.md` });
    }
    setSelected(new Set()); done();
  };

  const handleUrl = async () => {
    if (!url) return;
    setBusy(true);
    await createSource.mutateAsync({ title: url, content: `# URL Source\n\n${url}\n\n> Crawl pending.`, url, source_type: "url", raw_path: `raw/url-${Date.now()}.md` });
    setUrl(""); done();
  };

  const handleFile = async () => {
    if (!file) return;
    setBusy(true);
    const text = await file.text().catch(() => "");
    await createSource.mutateAsync({ title: file.name, content: text || `# ${file.name}\n\n> Binary file.`, source_type: "file", raw_path: `raw/${file.name}` });
    setFile(null); done();
  };

  const handleMd = async () => {
    if (!md.trim()) return;
    setBusy(true);
    const title = md.split("\n")[0]?.replace(/^#\s*/, "") || "Manual entry";
    await createSource.mutateAsync({ title, content: md, source_type: "manual", raw_path: `raw/manual-${Date.now()}.md` });
    setMd(""); done();
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

          <TabsContent value="inbox" className="space-y-4 pt-4">
            <p className="text-xs text-muted-foreground">Select inbox items to ingest as wiki sources.</p>
            {inboxLoading ? <Skeleton className="h-12 w-full" /> : (inboxItems?.length ?? 0) === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">No inbox items</p>
            ) : (
              <div className="max-h-64 space-y-1 overflow-y-auto">
                {(inboxItems || []).slice(0, 30).map((item) => (
                  <label key={item.id} className={`flex cursor-pointer gap-2 rounded border p-2 text-sm hover:bg-accent ${selected.has(item.id) ? "border-primary bg-accent" : ""}`}>
                    <input type="checkbox" className="mt-1" checked={selected.has(item.id)} onChange={() => toggle(item.id)} />
                    <div className="min-w-0 flex-1"><p className="truncate font-medium">{item.title || "(no subject)"}</p><p className="truncate text-xs text-muted-foreground">{item.body?.slice(0, 100)}</p></div>
                  </label>
                ))}
              </div>
            )}
            <Button onClick={handleInbox} disabled={selected.size === 0 || busy} className="w-full"><Download className="mr-2 h-4 w-4" />Ingest{selected.size > 0 ? ` (${selected.size})` : ""}</Button>
          </TabsContent>

          <TabsContent value="url" className="space-y-4 pt-4">
            <p className="text-xs text-muted-foreground">Enter a URL to crawl and extract knowledge.</p>
            <Input placeholder="https://example.com/article" value={url} onChange={(e) => setUrl(e.target.value)} />
            <Button onClick={handleUrl} disabled={!url || busy} className="w-full"><Globe className="mr-2 h-4 w-4" />Crawl & Ingest</Button>
          </TabsContent>

          <TabsContent value="file" className="space-y-4 pt-4">
            <p className="text-xs text-muted-foreground">Upload: .md .txt .pdf .xls .xlsx .doc .docx .wps</p>
            <Input type="file" accept=".md,.txt,.pdf,.xls,.xlsx,.doc,.docx,.wps" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
            {file && <p className="text-xs text-muted-foreground">{file.name} ({(file.size / 1024).toFixed(1)} KB)</p>}
            <Button onClick={handleFile} disabled={!file || busy} className="w-full"><FileUp className="mr-2 h-4 w-4" />Upload & Ingest</Button>
          </TabsContent>

          <TabsContent value="markdown" className="space-y-4 pt-4">
            <p className="text-xs text-muted-foreground">Write or paste markdown content.</p>
            <Textarea className="min-h-[200px] font-mono text-sm" placeholder="# Title\n\nWrite here..." value={md} onChange={(e) => setMd(e.target.value)} />
            <Button onClick={handleMd} disabled={!md.trim() || busy} className="w-full"><PenLine className="mr-2 h-4 w-4" />Save to Wiki</Button>
          </TabsContent>
        </Tabs>
        </div>
      </DialogContent>
    </Dialog>
  );
}
