"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { HardDrive, Search, Download, Trash2, Upload } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import type { OssProviderConfig, OssObject } from "@multica/core/types";

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function OssFileBrowser() {
  const wsId = useWorkspaceId();
  const [selectedConfig, setSelectedConfig] = useState("");
  const [search, setSearch] = useState("");
  const [uploading, setUploading] = useState(false);

  const { data: configs } = useQuery<OssProviderConfig[]>({
    queryKey: ["oss", "configs", wsId],
    queryFn: () => api.listOssConfigs(),
  });

  const { data: files, isLoading } = useQuery<OssObject[]>({
    queryKey: ["oss", "files", wsId, selectedConfig, search],
    queryFn: async () => {
      if (!selectedConfig) return [];
      const qs = new URLSearchParams();
      if (search) qs.set("prefix", search);
      return api.listOssFiles(selectedConfig, qs.toString() || undefined);
    },
    enabled: !!selectedConfig,
  });

  const handleDownload = async (file: OssObject) => {
    try {
      const resp = await api.getOssFileDownloadUrl(selectedConfig, file.id);
      window.open(resp.url, "_blank");
    } catch { alert("下载失败"); }
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !selectedConfig) return;
    setUploading(true);
    try {
      const form = new FormData();
      form.append("file", file);
      await api.uploadOssFile(selectedConfig, form);
      window.location.reload();
    } catch { alert("上传失败"); }
    setUploading(false);
  };

  const handleDelete = async (file: OssObject) => {
    if (!confirm(`删除 "${file.filename}"？`)) return;
    try {
      await api.deleteOssFile(selectedConfig, file.id);
      window.location.reload();
    } catch { alert("删除失败"); }
  };

  return (
    <div className="flex h-full flex-col p-6">
      <div className="mb-6 flex items-center gap-3">
        <HardDrive className="h-6 w-6 text-muted-foreground" />
        <h1 className="text-xl font-bold">云存储</h1>
      </div>

      {(!configs || configs.length === 0) ? (
        <div className="flex flex-1 items-center justify-center">
          <p className="text-sm text-muted-foreground">尚未配置对象存储。请在设置 &gt; 集成中添加 OSS 配置。</p>
        </div>
      ) : (
        <>
          <div className="mb-4 flex items-center gap-3">
            <select className="h-9 w-64 rounded-md border bg-background px-3 text-sm" value={selectedConfig} onChange={e => setSelectedConfig(e.target.value)}>
              <option value="">选择存储平台</option>
              {configs.map(c => <option key={c.id} value={c.id}>{c.name} ({c.bucket})</option>)}
            </select>
            <div className="relative flex-1 max-w-sm">
              <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input className="pl-8" placeholder="搜索文件 (prefix)..." value={search} onChange={e => setSearch(e.target.value)} />
            </div>
            <Button variant="outline" size="sm" disabled={uploading || !selectedConfig} onClick={() => document.getElementById("oss-upload")?.click()}>
              <Upload className="mr-1 h-3.5 w-3.5" />{uploading ? "上传中..." : "上传"}
            </Button>
            <input type="file" id="oss-upload" className="hidden" onChange={handleUpload} disabled={uploading || !selectedConfig} />
          </div>
          <div className="flex-1 overflow-y-auto rounded-md border">
            {isLoading ? <p className="p-8 text-center text-sm text-muted-foreground">加载中...</p>
            : !selectedConfig ? <p className="p-8 text-center text-sm text-muted-foreground">请选择存储平台</p>
            : files && files.length === 0 ? <p className="p-8 text-center text-sm text-muted-foreground">暂无文件</p>
            : (
              <table className="w-full text-sm">
                <thead className="border-b bg-muted/50 text-left">
                  <tr><th className="px-4 py-2 font-medium">文件名</th><th className="px-4 py-2 font-medium">Key</th><th className="px-4 py-2 font-medium w-24">大小</th><th className="px-4 py-2 font-medium w-40">上传时间</th><th className="px-4 py-2 font-medium w-24">操作</th></tr>
                </thead>
                <tbody className="divide-y">
                  {(files || []).map(f => (
                    <tr key={f.id} className="hover:bg-muted/30">
                      <td className="px-4 py-2 font-medium">{f.filename}</td>
                      <td className="px-4 py-2 font-mono text-xs text-muted-foreground">{f.key}</td>
                      <td className="px-4 py-2 text-muted-foreground">{formatBytes(f.size_bytes)}</td>
                      <td className="px-4 py-2 text-muted-foreground">{new Date(f.created_at).toLocaleString("zh-CN")}</td>
                      <td className="px-4 py-2"><div className="flex gap-1">
                        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => handleDownload(f)} title="下载"><Download className="h-3.5 w-3.5" /></Button>
                        <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive hover:text-destructive" onClick={() => handleDelete(f)} title="删除"><Trash2 className="h-3.5 w-3.5" /></Button>
                      </div></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </>
      )}
    </div>
  );
}
