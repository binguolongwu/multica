"use client";

import { useState, useMemo, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { HardDrive, Upload, FolderPlus, Trash2, ChevronRight, ChevronDown, Folder, FolderOpen, FileText, Search } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { toast } from "sonner";
import type { OssProviderConfig } from "@multica/core/types";

type TreeNode = { name: string; path: string; isDir: boolean; children: TreeNode[] };

function buildTree(keys: string[]): TreeNode[] {
  const root: TreeNode = { name: "/", path: "", isDir: true, children: [] };
  const m = new Map<string, TreeNode>([["", root]]);
  for (const k of [...keys].sort()) {
    const parts = k.split("/").filter(Boolean);
    let cur = "";
    for (let i = 0; i < parts.length; i++) {
      const p = parts[i]; if (!p) continue;
      const prev = cur; cur = cur ? `${cur}/${p}` : p;
      if (m.has(cur)) continue;
      const node: TreeNode = { name: p, path: cur, isDir: i < parts.length - 1 || k.endsWith("/"), children: [] };
      m.set(cur, node);
      m.get(prev)?.children.push(node);
    }
  }
  return root.children;
}

export function OssFileBrowser() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const [selectedConfig, setSelectedConfig] = useState("");
  const [search, setSearch] = useState("");
  const [newDirInput, setNewDirInput] = useState("");
  const [showNewDir, setShowNewDir] = useState(false);
  const [dragging, setDragging] = useState<string | null>(null);
  const [selectedDir, setSelectedDir] = useState("");

  const { data: configs } = useQuery<OssProviderConfig[]>({
    queryKey: ["oss", "configs", wsId],
    queryFn: () => api.listOssConfigs(),
  });

  const { data: keys, isLoading } = useQuery<string[]>({
    queryKey: ["oss", "keys", wsId, selectedConfig, search],
    queryFn: () => api.listOssFiles(selectedConfig, search ? `prefix=${encodeURIComponent(search)}` : undefined) as Promise<string[]>,
    enabled: !!selectedConfig,
  });

  const tree = useMemo(() => buildTree(keys || []), [keys]);

  const currentKey = (filename: string) => {
    const prefix = configs?.find(c => c.id === selectedConfig)?.folder_prefix || "";
    return selectedDir ? `${selectedDir}/${filename}` : filename;
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file || !selectedConfig) return;
    const form = new FormData(); form.append("file", file); form.append("key", currentKey(file.name));
    try { await api.uploadOssFile(selectedConfig, form); qc.invalidateQueries({ queryKey: ["oss", "keys", wsId, selectedConfig] }); }
    catch { toast.error("上传失败"); }
  };

  const handleCreateDir = async () => {
    const name = newDirInput.trim(); if (!name || !selectedConfig) return;
    const prefix = currentKey(name) + "/";
    try { await api.createOssDirectory(selectedConfig, prefix); qc.invalidateQueries({ queryKey: ["oss", "keys", wsId, selectedConfig] }); setShowNewDir(false); setNewDirInput(""); }
    catch { toast.error("创建目录失败"); }
  };

  const handleDeleteDir = async (dirPath: string) => {
    if (!selectedConfig || !confirm(`删除整个目录 "${dirPath}/" 及其所有文件？`)) return;
    try { await api.deleteOssDirectory(selectedConfig, dirPath); qc.invalidateQueries({ queryKey: ["oss", "keys", wsId, selectedConfig] }); }
    catch { toast.error("删除目录失败"); }
  };

  const handleDeleteFile = async (filePath: string) => {
    if (!selectedConfig) return;
    try {
      // Find the file ID from the oss_object table via the DB endpoint
      const resp = await api.listOssDbFiles(selectedConfig, `prefix=${encodeURIComponent(filePath)}`);
      const match = Array.isArray(resp) ? resp.find((f: any) => f.key === filePath) : null;
      if (match) await api.deleteOssFile(selectedConfig, match.id);
      qc.invalidateQueries({ queryKey: ["oss", "keys", wsId, selectedConfig] });
    } catch { toast.error("删除文件失败"); }
  };

  const handleDownload = async (filePath: string) => {
    if (!selectedConfig || !cfg) return;
    const domain = cfg.custom_domain || `${cfg.bucket}.${cfg.provider === "qiniu" ? "clouddn.com" : "com"}`;
    window.open(`https://${domain}/${filePath}`, "_blank");
  };

  const handleDrop = async (destDir: string) => {
    if (!selectedConfig || !dragging || dragging === destDir) { setDragging(null); return; }
    const name = dragging.split("/").pop() || "";
    const destKey = destDir ? `${destDir}/${name}` : name;
    try { await api.moveOssFile(selectedConfig, dragging, destKey); qc.invalidateQueries({ queryKey: ["oss", "keys", wsId, selectedConfig] }); }
    catch { toast.error("移动失败"); }
    setDragging(null);
  };

  const handleNodeClick = (node: TreeNode) => {
    setSelectedDir(node.isDir ? node.path : node.path);
  };

  const renderNode = (node: TreeNode, depth: number) => {
    const fullPath = node.path;
    const isSelected = selectedDir === fullPath;
    return (
      <div key={fullPath}>
        <button
          className={`flex w-full items-center gap-1 rounded px-2 py-1 text-left text-sm hover:bg-accent ${isSelected ? "bg-accent ring-1 ring-inset ring-border" : ""}`}
          style={{ paddingLeft: 12 + depth * 16 }}
          onClick={() => handleNodeClick(node)}
          draggable={!node.path.includes("/") || true}
          onDragStart={e => { setDragging(fullPath); e.dataTransfer.setData("text/plain", fullPath); }}
          onDragOver={e => { e.preventDefault(); }}
          onDrop={e => { e.preventDefault(); if (node.isDir) handleDrop(fullPath); }}
        >
          {node.isDir ? (
            node.name.endsWith("/") ? <Folder className="h-4 w-4 flex-shrink-0 text-muted-foreground" /> :
            node.children.length > 0 ? <FolderOpen className="h-4 w-4 flex-shrink-0 text-muted-foreground" /> :
            <Folder className="h-4 w-4 flex-shrink-0 text-muted-foreground" />
          ) : (
            <FileText className="h-4 w-4 flex-shrink-0 text-muted-foreground" />
          )}
          <span className={`truncate text-xs ${isSelected ? "font-medium" : ""}`}>{node.name}</span>
          <div className="ml-auto flex gap-0.5 shrink-0">
            {!node.isDir && <Button variant="ghost" size="icon" className="h-5 w-5" onClick={e => { e.stopPropagation(); handleDownload(fullPath); }} title="下载"><ChevronRight className="h-3 w-3" /></Button>}
            <Button variant="ghost" size="icon" className="h-5 w-5 text-destructive" onClick={e => { e.stopPropagation(); node.isDir ? handleDeleteDir(fullPath) : handleDeleteFile(fullPath); }} title="删除"><Trash2 className="h-3 w-3" /></Button>
          </div>
        </button>
        {node.isDir && node.children.map(c => renderNode(c, depth + 1))}
      </div>
    );
  };

  const cfg = configs?.find(c => c.id === selectedConfig);

  return (
    <div className="flex h-full flex-col p-6">
      <div className="mb-4 flex items-center gap-3 shrink-0">
        <HardDrive className="h-6 w-6 text-muted-foreground" />
        <h1 className="text-xl font-bold">云存储</h1>
      </div>

      {(!configs || configs.length === 0) ? (
        <p className="text-sm text-muted-foreground">尚未配置对象存储。请在设置 &gt; 集成中添加 OSS 配置。</p>
      ) : (
        <>
          <div className="mb-3 flex items-center gap-2 shrink-0 flex-wrap">
            <select className="h-9 w-48 rounded-md border bg-background px-3 text-sm" value={selectedConfig} onChange={e => { setSelectedConfig(e.target.value); setSelectedDir(""); }}>
              <option value="">选择存储平台</option>
              {configs.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
            <div className="relative w-48"><Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-8" placeholder="搜索..." value={search} onChange={e => setSearch(e.target.value)} /></div>
            <label><input type="file" className="hidden" onChange={handleUpload} disabled={!selectedConfig} /><Button variant="outline" size="sm" disabled={!selectedConfig} onClick={() => document.querySelector<HTMLInputElement>('input[type="file"]')?.click()}><Upload className="mr-1 h-3.5 w-3.5" />上传</Button></label>
            <Button variant="outline" size="sm" disabled={!selectedConfig} onClick={() => setShowNewDir(true)}><FolderPlus className="mr-1 h-3.5 w-3.5" />新建目录</Button>
          </div>
          {showNewDir && selectedConfig && (
            <div className="mb-3 flex items-center gap-2">
              <Input className="h-8 w-48" placeholder="目录名" value={newDirInput} onChange={e => setNewDirInput(e.target.value)} onKeyDown={e => { if (e.key === "Enter") handleCreateDir(); if (e.key === "Escape") setShowNewDir(false); }} autoFocus />
              <Button size="sm" onClick={handleCreateDir}>创建</Button>
              <Button size="sm" variant="outline" onClick={() => setShowNewDir(false)}>取消</Button>
            </div>
          )}
          {selectedConfig && cfg && (
            <div className="text-xs text-muted-foreground mb-2">{cfg.name} · {cfg.bucket} · {prefixForDisplay(cfg, selectedDir) || "/"}</div>
          )}
          <div className="flex-1 overflow-y-auto rounded-md border">
            {isLoading ? <p className="p-8 text-center text-sm text-muted-foreground">加载中...</p>
            : !selectedConfig ? <p className="p-8 text-center text-sm text-muted-foreground">请选择存储平台</p>
            : tree.length === 0 ? <p className="p-8 text-center text-sm text-muted-foreground">暂无文件</p>
            : tree.map(n => renderNode(n, 0))
            }
          </div>
        </>
      )}
    </div>
  );
}

function prefixForDisplay(cfg: OssProviderConfig, dir: string): string {
  const p = cfg.folder_prefix || "";
  return dir ? `${p}${dir}/` : (p || "/");
}
