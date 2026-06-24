"use client";

import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Cpu, Edit, Plus, Trash2, Loader2, ChevronRight } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import type { LLMProvider, LLMModel } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@multica/ui/components/ui/dialog";
import { toast } from "sonner";

const llmKeys = {
  providers: (wsId: string) => ["llm-providers", wsId] as const,
  models: (wsId: string, pid: string) => ["llm-models", wsId, pid] as const,
  templates: ["llm-provider-templates"] as const,
};

export function LlmSettingsTab() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const [selectedPid, setSelectedPid] = useState<string | null>(null);
  const [editingProvider, setEditingProvider] = useState<LLMProvider | null>(null);
  const [editingModel, setEditingModel] = useState<LLMModel | null>(null);
  const [providerDialog, setProviderDialog] = useState(false);
  const [modelDialog, setModelDialog] = useState(false);
  const [form, setForm] = useState({ name: "", code: "", api_type: "openai", api_base_url: "", api_key: "", env_var_api_key: "OPENAI_API_KEY", env_var_base_url: "OPENAI_BASE_URL", sort: 0 });
  const [modelForm, setModelForm] = useState({ name: "", model_code: "", type: 1, temperature: 0.7, max_tokens: 4096, context_window: 0, capabilities: [] as string[], sort: 0 });
  const [templateCode, setTemplateCode] = useState("");

  const providersQuery = useQuery({
    queryKey: llmKeys.providers(wsId),
    queryFn: () => api.listLLMProviders(wsId),
  });
  const templatesQuery = useQuery({
    queryKey: llmKeys.templates,
    queryFn: () => api.listLLMProviderTemplates(),
  });
  const modelsQuery = useQuery({
    queryKey: llmKeys.models(wsId, selectedPid || ""),
    queryFn: () => api.listLLMModels(wsId, selectedPid!),
    enabled: !!selectedPid,
  });

  const selectedProvider = useMemo(
    () => (providersQuery.data || []).find((p) => p.id === selectedPid) || null,
    [providersQuery.data, selectedPid],
  );

  // ── Provider Dialog ──────────────────────────────────────────────────────

  const openCreateProvider = () => {
    setEditingProvider(null);
    setTemplateCode("");
    setForm({ name: "", code: "", api_type: "openai", api_base_url: "", api_key: "", env_var_api_key: "OPENAI_API_KEY", env_var_base_url: "OPENAI_BASE_URL", sort: 0 });
    setProviderDialog(true);
  };

  const openEditProvider = (p: LLMProvider) => {
    setEditingProvider(p);
    setTemplateCode("");
    const isMasked = p.api_key.includes("****");
    setForm({ name: p.name, code: p.code, api_type: p.api_type, api_base_url: p.api_base_url, api_key: isMasked ? "" : p.api_key, env_var_api_key: p.env_var_api_key, env_var_base_url: p.env_var_base_url, sort: p.sort });
    setProviderDialog(true);
  };

  const applyTemplate = (code: string) => {
    const tpl = (templatesQuery.data || []).find((t) => t.code === code);
    if (!tpl) return;
    setTemplateCode(code);
    setForm({
      ...form,
      name: tpl.name,
      code: tpl.code,
      api_type: tpl.api_type,
      api_base_url: tpl.anthropic_api_url || tpl.api_base_url,
      env_var_api_key: tpl.env_var_api_key,
      env_var_base_url: tpl.env_var_base_url,
      api_key: "",
    });
  };

  const saveProviderMutation = useMutation({
    mutationFn: async () => {
      const payload = editingProvider && !form.api_key
        ? { name: form.name, code: form.code, api_type: form.api_type, api_base_url: form.api_base_url, env_var_api_key: form.env_var_api_key, env_var_base_url: form.env_var_base_url, sort: form.sort }
        : form;
      if (editingProvider) return api.updateLLMProvider(wsId, editingProvider.id, payload);
      return api.createLLMProvider(wsId, payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: llmKeys.providers(wsId) });
      qc.invalidateQueries({ queryKey: llmKeys.templates });
      toast.success(editingProvider ? "Provider updated" : "Provider created");
      setProviderDialog(false);
    },
    onError: () => toast.error("Failed to save provider"),
  });

  const deleteProviderMutation = useMutation({
    mutationFn: (id: string) => api.deleteLLMProvider(wsId, id),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: llmKeys.providers(wsId) });
      if (selectedPid === id) setSelectedPid(null);
      toast.success("Provider deleted");
    },
    onError: () => toast.error("Failed to delete provider"),
  });

  // ── Model Dialog ─────────────────────────────────────────────────────────

  const openCreateModel = () => {
    setEditingModel(null);
    setModelForm({ name: "", model_code: "", type: 1, temperature: 0.7, max_tokens: 4096, context_window: 0, capabilities: [], sort: 0 });
    setModelDialog(true);
  };

  const openEditModel = (m: LLMModel) => {
    setEditingModel(m);
    setModelForm({ name: m.name, model_code: m.model_code, type: m.type, temperature: m.temperature, max_tokens: m.max_tokens, context_window: m.context_window, capabilities: m.capabilities || [], sort: m.sort });
    setModelDialog(true);
  };

  const saveModelMutation = useMutation({
    mutationFn: async () => {
      if (editingModel) return api.updateLLMModel(wsId, selectedPid!, editingModel.id, modelForm);
      return api.createLLMModel(wsId, selectedPid!, modelForm);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: llmKeys.models(wsId, selectedPid!) });
      toast.success(editingModel ? "Model updated" : "Model created");
      setModelDialog(false);
    },
    onError: () => toast.error("Failed to save model"),
  });

  const deleteModelMutation = useMutation({
    mutationFn: (id: string) => api.deleteLLMModel(wsId, selectedPid!, id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: llmKeys.models(wsId, selectedPid!) });
      toast.success("Model deleted");
    },
    onError: () => toast.error("Failed to delete model"),
  });

  // ── Render ───────────────────────────────────────────────────────────────

  const providers = providersQuery.data || [];
  const templates = templatesQuery.data || [];
  const models = modelsQuery.data || [];

  return (
    <div className="flex flex-1 min-h-0 gap-0">
      {/* Left: Provider list */}
      <div className="w-56 shrink-0 border-r overflow-y-auto p-3 space-y-2">
        <h2 className="text-sm font-semibold px-1">供应商</h2>
        {providersQuery.isLoading && <Loader2 className="h-4 w-4 animate-spin mx-auto mt-4" />}
        {providers.map((p) => (
          <button
            key={p.id}
            onClick={() => setSelectedPid(p.id)}
            className={`w-full text-left px-2 py-1.5 rounded text-sm flex items-center justify-between ${selectedPid === p.id ? "bg-accent font-medium" : "hover:bg-accent/50"}`}
          >
            <span className="truncate">{p.name}</span>
            <ChevronRight className="h-3 w-3 opacity-40" />
          </button>
        ))}
        <Button variant="outline" size="sm" className="w-full" onClick={openCreateProvider}>
          <Plus className="h-3 w-3 mr-1" />新增供应商
        </Button>
      </div>

      {/* Right: Model list */}
      <div className="flex-1 min-w-0 overflow-y-auto p-4">
        {!selectedPid && (
          <p className="text-sm text-muted-foreground text-center mt-12">请从左侧选择一个供应商</p>
        )}

        {selectedPid && selectedProvider && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="font-semibold">{selectedProvider.name}</h3>
                <p className="text-xs text-muted-foreground font-mono">{selectedProvider.api_base_url}</p>
              </div>
              <div className="flex gap-1">
                <Button variant="ghost" size="icon-sm" onClick={() => openEditProvider(selectedProvider)}><Edit className="h-4 w-4" /></Button>
                <Button variant="ghost" size="icon-sm" onClick={() => deleteProviderMutation.mutate(selectedProvider.id)}><Trash2 className="h-4 w-4" /></Button>
              </div>
            </div>

            {modelsQuery.isLoading && <Loader2 className="h-4 w-4 animate-spin" />}

            <div className="space-y-2">
              {models.map((m) => (
                <Card key={m.id}>
                  <CardContent className="flex items-center justify-between py-2 px-3">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-sm">{m.name}</span>
                        <span className="text-xs text-muted-foreground font-mono">{m.model_code}</span>
                        <span className="text-xs px-1 rounded bg-muted">{m.type === 1 ? "LLM" : m.type === 2 ? "视觉" : `类型${m.type}`}</span>
                      </div>
                      <div className="text-xs text-muted-foreground mt-0.5">
                        temp: {m.temperature} | max tokens: {m.max_tokens} | context: {m.context_window > 0 ? `${m.context_window}` : "-"}
                      </div>
                    </div>
                    <div className="flex gap-1 shrink-0">
                      <Button variant="ghost" size="icon-sm" onClick={() => openEditModel(m)}><Edit className="h-3.5 w-3.5" /></Button>
                      <Button variant="ghost" size="icon-sm" onClick={() => deleteModelMutation.mutate(m.id)}><Trash2 className="h-3.5 w-3.5" /></Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
              {!modelsQuery.isLoading && models.length === 0 && (
                <p className="text-sm text-muted-foreground italic">暂无模型</p>
              )}
            </div>

            <Button variant="outline" size="sm" onClick={openCreateModel}>
              <Plus className="h-3 w-3 mr-1" />新增模型
            </Button>
          </div>
        )}
      </div>

      {/* ── Provider Dialog ────────────── */}
      <Dialog open={providerDialog} onOpenChange={setProviderDialog}>
        <DialogContent className="w-[60vw] max-h-[60vh] overflow-y-auto">
          <DialogTitle>{editingProvider ? "编辑供应商" : "新增供应商"}</DialogTitle>
          <div className="space-y-3">
            <div>
              <label className="text-xs font-medium">选择模板</label>
              <select
                className="w-full border rounded px-2 py-1.5 text-sm mt-1"
                value={templateCode}
                onChange={(e) => applyTemplate(e.target.value)}
              >
                <option value="">-- 手动输入 --</option>
                {templates.map((t) => (
                  <option key={t.code} value={t.code}>{t.name} ({t.api_type})</option>
                ))}
              </select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs font-medium">名称 *</label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="e.g. DeepSeek" />
              </div>
              <div>
                <label className="text-xs font-medium">编码 *</label>
                <Input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} placeholder="e.g. deepseek" />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs font-medium">API Base URL</label>
                <Input value={form.api_base_url} onChange={(e) => setForm({ ...form, api_base_url: e.target.value })} placeholder="https://api.deepseek.com/anthropic" />
              </div>
              <div>
                <label className="text-xs font-medium">API Key</label>
                <Input type="password" value={form.api_key} onChange={(e) => setForm({ ...form, api_key: e.target.value })} placeholder={editingProvider ? "（不修改则留空）" : "sk-..."} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs font-medium">Env Var (Key)</label>
                <Input value={form.env_var_api_key} onChange={(e) => setForm({ ...form, env_var_api_key: e.target.value })} />
              </div>
              <div>
                <label className="text-xs font-medium">Env Var (Base URL)</label>
                <Input value={form.env_var_base_url} onChange={(e) => setForm({ ...form, env_var_base_url: e.target.value })} />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setProviderDialog(false)}>取消</Button>
            <Button size="sm" onClick={() => saveProviderMutation.mutate()} disabled={!form.name || !form.code || saveProviderMutation.isPending}>
              {saveProviderMutation.isPending && <Loader2 className="h-3 w-3 animate-spin mr-1" />}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Model Dialog ────────────── */}
      <Dialog open={modelDialog} onOpenChange={setModelDialog}>
        <DialogContent className="sm:max-w-md">
          <DialogTitle>{editingModel ? "编辑模型" : "新增模型"}</DialogTitle>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs font-medium">名称 *</label>
                <Input value={modelForm.name} onChange={(e) => setModelForm({ ...modelForm, name: e.target.value })} placeholder="e.g. DeepSeek-V4-Pro" />
              </div>
              <div>
                <label className="text-xs font-medium">编码 *</label>
                <Input value={modelForm.model_code} onChange={(e) => setModelForm({ ...modelForm, model_code: e.target.value })} placeholder="deepseek-v4-pro" />
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <label className="text-xs font-medium">类型</label>
                <select className="w-full border rounded px-2 py-1.5 text-sm" value={modelForm.type} onChange={(e) => setModelForm({ ...modelForm, type: Number(e.target.value) })}>
                  <option value={1}>LLM 对话</option>
                  <option value={2}>视觉</option>
                  <option value={3}>生图</option>
                  <option value={4}>嵌入</option>
                  <option value={5}>语音</option>
                </select>
              </div>
              <div>
                <label className="text-xs font-medium">Temperature</label>
                <Input type="number" step="0.1" min="0" max="2" value={modelForm.temperature} onChange={(e) => setModelForm({ ...modelForm, temperature: Number(e.target.value) })} />
              </div>
              <div>
                <label className="text-xs font-medium">Max Tokens</label>
                <Input type="number" value={modelForm.max_tokens} onChange={(e) => setModelForm({ ...modelForm, max_tokens: Number(e.target.value) })} />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setModelDialog(false)}>取消</Button>
            <Button size="sm" onClick={() => saveModelMutation.mutate()} disabled={!modelForm.name || !modelForm.model_code || saveModelMutation.isPending}>
              {saveModelMutation.isPending && <Loader2 className="h-3 w-3 animate-spin mr-1" />}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
