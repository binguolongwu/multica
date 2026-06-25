"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, Trash2, Server } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@multica/ui/components/ui/dialog";
import type { OssProviderConfig, CreateOssConfigRequest } from "@multica/core/types";

const PROVIDER_LABELS: Record<string, string> = {
  qiniu: "七牛云 Kodo",
  aliyun_oss: "阿里云 OSS",
  tencent_cos: "腾讯云 COS",
  huawei_obs: "华为云 OBS",
  baidu_bos: "百度智能云 BOS",
  volcengine_tos: "火山引擎 TOS",
  s3_compatible: "S3 兼容存储",
  minio: "MinIO 自建",
};

function providerLabel(provider: string) { return PROVIDER_LABELS[provider] || provider; }

const ossKeys = { configs: (wsId: string) => ["oss", "configs", wsId] as const };

export function OssSettingsTab() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<OssProviderConfig | null>(null);
  const [testing, setTesting] = useState(false);
  const [form, setForm] = useState<CreateOssConfigRequest>({ name: "", provider: "s3_compatible", bucket: "", region: "", endpoint: "", access_key: "", secret_key: "", custom_domain: "", folder_prefix: "" });

  const { data: configs, isLoading } = useQuery<OssProviderConfig[]>({
    queryKey: ossKeys.configs(wsId),
    queryFn: () => api.listOssConfigs(),
  });

  const openCreate = () => { setEditing(null); setForm({ name: "", provider: "s3_compatible", bucket: "", region: "", endpoint: "", access_key: "", secret_key: "", custom_domain: "", folder_prefix: "" }); setDialogOpen(true); };
  const openEdit = (c: OssProviderConfig) => { setEditing(c); setForm({ name: c.name, provider: c.provider, bucket: c.bucket, region: c.region, endpoint: c.endpoint, access_key: c.access_key, secret_key: "", custom_domain: c.custom_domain, folder_prefix: c.folder_prefix }); setDialogOpen(true); };

  const handleSave = async () => {
    try {
      if (editing) {
        await api.updateOssConfig(editing.id, form);
      } else {
        await api.createOssConfig(form);
      }
      qc.invalidateQueries({ queryKey: ossKeys.configs(wsId) });
      setDialogOpen(false);
      toast.success(editing ? "配置已更新" : "配置已创建");
    } catch { toast.error("保存失败"); }
  };

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`删除 OSS 配置 "${name}"？此操作不可撤销。`)) return;
    try {
      await api.deleteOssConfig(id);
      qc.invalidateQueries({ queryKey: ossKeys.configs(wsId) });
      toast.success("已删除");
    } catch { toast.error("删除失败"); }
  };

  return (
    <div className="space-y-4">
      {isLoading ? (
        <p className="text-xs text-muted-foreground">加载中...</p>
      ) : (
        <>
          {(configs || []).map((c) => (
            <Card key={c.id}>
              <CardContent className="flex items-center gap-3 p-4">
                <Server className="h-5 w-5 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{c.name}</p>
                  <p className="truncate text-xs text-muted-foreground">{providerLabel(c.provider)} · {c.bucket}{c.custom_domain ? ` · ${c.custom_domain}` : ""}</p>
                  {c.folder_prefix && <p className="text-xs text-muted-foreground">prefix: {c.folder_prefix}</p>}
                </div>
                <Button variant="outline" size="sm" onClick={() => openEdit(c)}>编辑</Button>
                <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" onClick={() => handleDelete(c.id, c.name)}><Trash2 className="h-4 w-4" /></Button>
              </CardContent>
            </Card>
          ))}
          {(configs || []).length === 0 && <p className="text-xs text-muted-foreground">尚未配置对象存储</p>}
          <Button variant="outline" size="sm" onClick={openCreate}><Plus className="mr-1 h-3.5 w-3.5" />添加 OSS 配置</Button>
        </>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="!max-w-lg">
          <DialogHeader><DialogTitle>{editing ? "编辑 OSS 配置" : "添加 OSS 配置"}</DialogTitle></DialogHeader>
          <div className="grid grid-cols-2 gap-3 max-h-[60vh] overflow-y-auto">
            <div className="col-span-2"><Label>名称</Label><Input value={form.name} onChange={e => setForm({...form, name: e.target.value})} placeholder="生产环境七牛" /></div>
            <div className="col-span-2">
              <Label>服务商</Label>
              <select className="w-full h-9 rounded-md border bg-background px-3 text-sm" value={form.provider} onChange={e => setForm({...form, provider: e.target.value})}>
                {Object.entries(PROVIDER_LABELS).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
              </select>
            </div>
            <div className="col-span-2"><Label>Bucket</Label><Input value={form.bucket} onChange={e => setForm({...form, bucket: e.target.value})} placeholder="my-bucket" /></div>
            <div><Label>Region</Label><Input value={form.region} onChange={e => setForm({...form, region: e.target.value})} placeholder="cn-beijing" /></div>
            <div><Label>Endpoint (可选)</Label><Input value={form.endpoint} onChange={e => setForm({...form, endpoint: e.target.value})} placeholder="https://oss-cn-beijing.aliyuncs.com" /></div>
            <div className="col-span-2"><Label>AccessKey</Label><Input value={form.access_key} onChange={e => setForm({...form, access_key: e.target.value})} /></div>
            <div className="col-span-2"><Label>SecretKey {editing ? "(留空不修改)" : ""}</Label><Input type="password" value={form.secret_key} onChange={e => setForm({...form, secret_key: e.target.value})} /></div>
            <div><Label>自定义域名 (可选)</Label><Input value={form.custom_domain} onChange={e => setForm({...form, custom_domain: e.target.value})} placeholder="cdn.example.com" /></div>
            <div><Label>目录前缀 (可选)</Label><Input value={form.folder_prefix} onChange={e => setForm({...form, folder_prefix: e.target.value})} placeholder="multica/" /></div>
          </div>
          <DialogFooter className="flex gap-2">
            <Button variant="outline" size="sm" onClick={async () => {
              setTesting(true);
              try {
                if (editing) {
                  await api.testOssConfigConnection(editing.id);
                } else {
                  await api.testOssConnection(form);
                }
                toast.success("连接测试成功");
              } catch { toast.error("连接测试失败，请检查参数"); }
              setTesting(false);
            }} disabled={!form.bucket || !form.access_key || testing}>
              {testing ? "测试中..." : "测试连接"}
            </Button>
            <div className="flex-1" />
            <Button variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
            <Button onClick={handleSave} disabled={!form.name || !form.bucket || !form.access_key}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
