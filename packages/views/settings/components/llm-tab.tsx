"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Edit, Plus, Trash2, Loader2, ChevronRight, RefreshCw, Search } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { LLM_CAPABILITIES, CAPABILITY_LABELS } from "@multica/core/types";
import type { LLMProvider, LLMModel, LLMModelCandidate, LLMProviderEndpoint } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Switch } from "@multica/ui/components/ui/switch";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@multica/ui/components/ui/dialog";
import { CapabilityBadges } from "../../common/capability-badges";
import { LLMEndpointEditor } from "./llm-endpoint-editor";
import { toast } from "sonner";

const llmKeys = {
  providers: (wsId: string) => ["llm-providers", wsId] as const,
  models: (wsId: string, pid: string) => ["llm-models", wsId, pid] as const,
  templates: ["llm-provider-templates"] as const,
};

// currencySymbol maps the stored ISO currency code (CNY/USD/EUR) to the symbol
// used when rendering amounts (¥/$/€). Unknown/legacy codes fall back to ¥.
function currencySymbol(code?: string): string {
  if (code === "USD") return "$";
  if (code === "EUR") return "€";
  return "¥";
}

export function LlmSettingsTab() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const [selectedPid, setSelectedPid] = useState<string | null>(null);
  const [editingProvider, setEditingProvider] = useState<LLMProvider | null>(null);
  const [editingModel, setEditingModel] = useState<LLMModel | null>(null);
  const [providerDialog, setProviderDialog] = useState(false);
  const [modelDialog, setModelDialog] = useState(false);
  const [form, setForm] = useState({ name: "", code: "", api_key: "", sort: 0 });
  const [modelForm, setModelForm] = useState({ name: "", model_code: "", temperature: 0.7, max_tokens: 4096, context_window: 0, capabilities: [] as string[], sort: 0, currency: "CNY", input_price: 0, output_price: 0 });
  const [templateCode, setTemplateCode] = useState("");
  // Fetch-models dialog: provider returns candidates; the user multi-selects
  // which to import (avoids dumping the whole remote catalog into the DB).
  const [fetchDialogOpen, setFetchDialogOpen] = useState(false);
  const [candidates, setCandidates] = useState<LLMModelCandidate[]>([]);
  const [selectedCodes, setSelectedCodes] = useState<Set<string>>(new Set());
  const [candidateSearch, setCandidateSearch] = useState("");

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

  // editingProviderLive is the real-time provider data for the provider being
  // edited in the dialog. Unlike the editingProvider snapshot (captured at
  // dialog open), this reflects the latest React Query data so the endpoint
  // list refreshes immediately after add/delete/update operations.
  const editingProviderLive = useMemo(
    () => editingProvider
      ? (providersQuery.data || []).find((p) => p.id === editingProvider.id) || editingProvider
      : null,
    [editingProvider, providersQuery.data],
  );

  // ── Provider Dialog ──────────────────────────────────────────────────────

  const openCreateProvider = () => {
    setEditingProvider(null);
    setTemplateCode("");
    setForm({ name: "", code: "", api_key: "", sort: 0 });
    setProviderDialog(true);
  };

  const openEditProvider = (p: LLMProvider) => {
    setEditingProvider(p);
    setTemplateCode("");
    const isMasked = p.api_key.includes("****");
    setForm({ name: p.name, code: p.code, api_key: isMasked ? "" : p.api_key, sort: p.sort });
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
      api_key: "",
    });
  };

  const saveProviderMutation = useMutation({
    mutationFn: async () => {
      const payload = editingProvider && !form.api_key
        ? { name: form.name, code: form.code, sort: form.sort }
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
    setModelForm({ name: "", model_code: "", temperature: 0.7, max_tokens: 4096, context_window: 0, capabilities: [], sort: 0, currency: "CNY", input_price: 0, output_price: 0 });
    setModelDialog(true);
  };

  const openEditModel = (m: LLMModel) => {
    setEditingModel(m);
    setModelForm({ name: m.name, model_code: m.model_code, temperature: m.temperature, max_tokens: m.max_tokens, context_window: m.context_window, capabilities: m.capabilities || [], sort: m.sort, currency: m.currency || "CNY", input_price: m.input_price ?? 0, output_price: m.output_price ?? 0 });
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

  // Fetch remote model candidates (NOT persisted) and open the multi-select
  // dialog. Only the user's selection is imported via importModelsMutation —
  // avoids dumping the whole remote catalog (e.g. 66 models) into the DB.
  const fetchCandidatesMutation = useMutation({
    mutationFn: () => api.fetchProviderModels(wsId, selectedPid!),
    onSuccess: (list) => {
      setCandidates(list);
      setSelectedCodes(new Set());
      setCandidateSearch("");
      setFetchDialogOpen(true);
    },
    onError: () => toast.error("获取模型列表失败"),
  });

  // Import the user-selected candidates (upsert). Backend returns the full
  // refreshed model list for the provider.
  const importModelsMutation = useMutation({
    mutationFn: (selected: LLMModelCandidate[]) =>
      api.importLLMModels(wsId, selectedPid!, selected),
    onSuccess: (models) => {
      qc.invalidateQueries({ queryKey: llmKeys.models(wsId, selectedPid!) });
      setFetchDialogOpen(false);
      toast.success(`已导入 ${models.length} 个模型`);
    },
    onError: () => toast.error("导入模型失败"),
  });

  // Toggle a model's enabled state (status 1/0) via the existing PUT endpoint.
  const toggleModelMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.updateLLMModel(wsId, selectedPid!, id, { status: enabled ? 1 : 0 }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: llmKeys.models(wsId, selectedPid!) });
    },
    onError: () => toast.error("Failed to update model status"),
  });

  // ── Fetch dialog helpers ───────────────────────────────────────────────
  const existingCodes = useMemo(
    () => new Set((modelsQuery.data ?? []).map((m) => m.model_code)),
    [modelsQuery.data],
  );
  const filteredCandidates = useMemo(() => {
    const needle = candidateSearch.trim().toLowerCase();
    return candidates.filter((c) => {
      // Hide models already imported into the DB — only offer new ones.
      if (existingCodes.has(c.model_code)) return false;
      if (!needle) return true;
      return c.model_code.toLowerCase().includes(needle) || c.name.toLowerCase().includes(needle);
    });
  }, [candidates, candidateSearch, existingCodes]);
  const selectedCandidates = useMemo(
    () => candidates.filter((c) => selectedCodes.has(c.model_code)),
    [candidates, selectedCodes],
  );

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
                <p className="text-xs text-muted-foreground">
                  {((selectedProvider as any).endpoints || []).length} 个端点 · {models.length} 个模型
                </p>
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
                      </div>
                      <div className="text-xs text-muted-foreground mt-0.5">
                        temp: {m.temperature} | max tokens: {m.max_tokens} | context: {m.context_window > 0 ? `${m.context_window}` : "-"}
                      </div>
                      <div className="text-xs text-muted-foreground mt-0.5">
                        {m.input_price > 0 || m.output_price > 0
                          ? `价格: ${currencySymbol(m.currency)}${m.input_price} / ${currencySymbol(m.currency)}${m.output_price} 每百万token`
                          : "价格: 未定价"}
                      </div>
                      <CapabilityBadges capabilities={m.capabilities} className="mt-1" />
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Switch
                        checked={m.status === 1}
                        onCheckedChange={(v) => toggleModelMutation.mutate({ id: m.id, enabled: v })}
                        aria-label={`启用 ${m.name}`}
                      />
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

            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => fetchCandidatesMutation.mutate()} disabled={fetchCandidatesMutation.isPending}>
                {fetchCandidatesMutation.isPending ? <Loader2 className="h-3 w-3 mr-1 animate-spin" /> : <RefreshCw className="h-3 w-3 mr-1" />}
                获取模型列表
              </Button>
              <Button variant="outline" size="sm" onClick={openCreateModel}>
                <Plus className="h-3 w-3 mr-1" />新增模型
              </Button>
            </div>
          </div>
        )}
      </div>

      {/* ── Provider Dialog ────────────── */}
      <Dialog open={providerDialog} onOpenChange={setProviderDialog}>
        <DialogContent className="overflow-y-auto" style={{ width: "60vw", maxWidth: "60vw", maxHeight: "70vh" }}>
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
                  <option key={t.code} value={t.code}>{t.name} ({t.code})</option>
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
            <div>
              <label className="text-xs font-medium">API Key</label>
              <Input type="password" value={form.api_key} onChange={(e) => setForm({ ...form, api_key: e.target.value })} placeholder={editingProvider ? "（不修改则留空）" : "sk-..."} />
              <p className="text-[11px] text-muted-foreground mt-1">供应商级别密钥，所有端点共用。</p>
            </div>
            {editingProviderLive && (
              <LLMEndpointEditor
                workspaceId={wsId}
                providerId={editingProviderLive.id}
                endpoints={(editingProviderLive as any).endpoints || []}
              />
            )}
            {editingProvider && !editingProviderLive && (
              <p className="text-xs text-muted-foreground italic">保存后可添加 API 端点</p>
            )}
            {!editingProvider && (
              <p className="text-xs text-muted-foreground italic">保存后可添加 API 端点</p>
            )}
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
        <DialogContent className="overflow-y-auto" style={{ width: "60vw", maxWidth: "60vw", maxHeight: "60vh" }}>
          <DialogTitle>{editingModel ? "编辑模型" : "新增模型"}</DialogTitle>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs font-medium">名称 *</label>
                <Input
                  className="mt-1"
                  value={modelForm.name}
                  onChange={(e) => {
                    setModelForm({ ...modelForm, name: e.target.value, model_code: e.target.value });
                  }}
                  placeholder="e.g. DeepSeek-V4-Pro"
                />
              </div>
              <div>
                <label className="text-xs font-medium">编码 *</label>
                <Input value={modelForm.model_code} onChange={(e) => setModelForm({ ...modelForm, model_code: e.target.value })} placeholder="deepseek-v4-pro" />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs font-medium">Temperature</label>
                <Input type="number" step="0.1" min="0" max="2" value={modelForm.temperature} onChange={(e) => setModelForm({ ...modelForm, temperature: Number(e.target.value) })} />
              </div>
              <div>
                <label className="text-xs font-medium">Max Tokens</label>
                <Input type="number" value={modelForm.max_tokens} onChange={(e) => setModelForm({ ...modelForm, max_tokens: Number(e.target.value) })} />
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <label className="text-xs font-medium">币种</label>
                <select className="w-full border rounded px-2 py-1.5 text-sm mt-1" value={modelForm.currency} onChange={(e) => setModelForm({ ...modelForm, currency: e.target.value })}>
                  <option value="CNY">¥ CNY 人民币</option>
                  <option value="USD">$ USD 美元</option>
                  <option value="EUR">€ EUR 欧元</option>
                </select>
              </div>
              <div>
                <label className="text-xs font-medium">输入价格 (每百万token)</label>
                <Input className="mt-1" type="number" step="0.01" min="0" value={modelForm.input_price} onChange={(e) => setModelForm({ ...modelForm, input_price: Number(e.target.value) })} placeholder="0.00" />
              </div>
              <div>
                <label className="text-xs font-medium">输出价格 (每百万token)</label>
                <Input className="mt-1" type="number" step="0.01" min="0" value={modelForm.output_price} onChange={(e) => setModelForm({ ...modelForm, output_price: Number(e.target.value) })} placeholder="0.00" />
              </div>
            </div>
            <div>
              <label className="text-xs font-medium">能力</label>
              <div className="flex flex-wrap gap-x-3 gap-y-1.5 mt-1">
                {LLM_CAPABILITIES.map((c) => (
                  <label key={c} className="flex items-center gap-1 text-xs cursor-pointer select-none">
                    <Checkbox
                      checked={modelForm.capabilities.includes(c)}
                      onCheckedChange={(v) => {
                        setModelForm((prev) => ({
                          ...prev,
                          capabilities: v
                            ? [...prev.capabilities, c]
                            : prev.capabilities.filter((x) => x !== c),
                        }));
                      }}
                    />
                    {CAPABILITY_LABELS[c]}
                  </label>
                ))}
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

      {/* ── Fetch Models Dialog (multi-select import) ────────────── */}
      <Dialog open={fetchDialogOpen} onOpenChange={setFetchDialogOpen}>
        <DialogContent className="overflow-y-auto" style={{ width: "60vw", maxWidth: "60vw", maxHeight: "70vh" }}>
          <DialogTitle>获取模型列表 — 选择要导入的模型</DialogTitle>
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                className="pl-7 h-8"
                placeholder="搜索模型..."
                value={candidateSearch}
                onChange={(e) => setCandidateSearch(e.target.value)}
              />
            </div>
            <Button
              variant="outline"
              size="sm"
              disabled={filteredCandidates.length === 0}
              onClick={() => {
                const allSel = filteredCandidates.every((c) => selectedCodes.has(c.model_code));
                setSelectedCodes((prev) => {
                  const next = new Set(prev);
                  for (const c of filteredCandidates) {
                    if (allSel) next.delete(c.model_code);
                    else next.add(c.model_code);
                  }
                  return next;
                });
              }}
            >
              {filteredCandidates.length > 0 && filteredCandidates.every((c) => selectedCodes.has(c.model_code)) ? "清空筛选" : "全选筛选"}
            </Button>
          </div>

          <div className="max-h-[55vh] overflow-y-auto space-y-1 mt-2">
            {filteredCandidates.map((c) => {
              const checked = selectedCodes.has(c.model_code);
              return (
                <label
                  key={c.model_code}
                  className={`flex items-start gap-2 rounded-md px-2 py-1.5 cursor-pointer ${checked ? "bg-accent" : "hover:bg-accent/50"}`}
                >
                  <Checkbox
                    className="mt-0.5"
                    checked={checked}
                    onCheckedChange={(v) => {
                      setSelectedCodes((prev) => {
                        const next = new Set(prev);
                        if (v === true) next.add(c.model_code);
                        else next.delete(c.model_code);
                        return next;
                      });
                    }}
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-sm truncate">{c.name}</span>
                      <span className="text-xs text-muted-foreground font-mono truncate">{c.model_code}</span>
                    </div>
                    <CapabilityBadges capabilities={c.capabilities} className="mt-1" />
                    <div className="text-xs text-muted-foreground mt-0.5">
                      {c.context_window > 0 ? `context: ${c.context_window}` : "context: -"}
                      {" | "}
                      {c.input_price > 0 || c.output_price > 0
                        ? `价格: ${currencySymbol(c.currency)}${c.input_price} / ${currencySymbol(c.currency)}${c.output_price}`
                        : "未定价"}
                    </div>
                  </div>
                </label>
              );
            })}
            {filteredCandidates.length === 0 && (
              <p className="text-sm text-muted-foreground text-center py-6">无匹配模型</p>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setFetchDialogOpen(false)}>取消</Button>
            <Button
              size="sm"
              disabled={selectedCandidates.length === 0 || importModelsMutation.isPending}
              onClick={() => importModelsMutation.mutate(selectedCandidates)}
            >
              {importModelsMutation.isPending && <Loader2 className="h-3 w-3 animate-spin mr-1" />}
              导入{selectedCandidates.length > 0 ? `(${selectedCandidates.length})` : ""}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
