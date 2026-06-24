"use client";

import { useCallback, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Cpu, Edit, Plus, Trash2, Loader2 } from "lucide-react";
import { api } from "@multica/core/api";
import type { LLMProvider, LLMModel } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@multica/ui/components/ui/dialog";
import { toast } from "sonner";
import { useT } from "../../i18n";

const llmKeys = {
  providers: ["llm-providers"] as const,
  models: ["llm-models"] as const,
};

export function LlmSettingsTab() {
  const { t } = useT("settings");
  const qc = useQueryClient();
  const [editing, setEditing] = useState<LLMProvider | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState({ name: "", api_base_url: "", api_key: "", env_var_api_key: "ANTHROPIC_API_KEY", env_var_base_url: "ANTHROPIC_BASE_URL" });

  const providersQuery = useQuery({
    queryKey: llmKeys.providers,
    queryFn: () => api.listLLMProviders(),
  });
  const modelsQuery = useQuery({
    queryKey: llmKeys.models,
    queryFn: () => api.listLLMModels(),
  });

  const modelsByProvider = useCallback(
    (providerId: string) =>
      (modelsQuery.data ?? []).filter((m) => m.provider_id === providerId),
    [modelsQuery.data],
  );

  const openCreate = () => {
    setEditing(null);
    setForm({ name: "", api_base_url: "", api_key: "", env_var_api_key: "ANTHROPIC_API_KEY", env_var_base_url: "ANTHROPIC_BASE_URL" });
    setDialogOpen(true);
  };
  const openEdit = (p: LLMProvider) => {
    setEditing(p);
    // If api_key is masked (contains ****), don't pre-fill — the user
    // must type a new key to change it. The backend preserves the
    // existing value when we send an empty api_key on update.
    const isMasked = p.api_key.includes("****");
    setForm({
      name: p.name,
      api_base_url: p.api_base_url,
      api_key: isMasked ? "" : p.api_key,
      env_var_api_key: p.env_var_api_key,
      env_var_base_url: p.env_var_base_url,
    });
    setDialogOpen(true);
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      // When editing and api_key is empty (masked), omit it so the
      // backend COALESCE preserves the stored value.
      const payload = editing && !form.api_key
        ? { name: form.name, api_base_url: form.api_base_url, env_var_api_key: form.env_var_api_key, env_var_base_url: form.env_var_base_url }
        : form;
      if (editing) {
        return api.updateLLMProvider(editing.id, payload);
      }
      return api.createLLMProvider(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: llmKeys.providers });
      toast.success(editing ? "Provider updated" : "Provider created");
      setDialogOpen(false);
    },
    onError: () => toast.error("Failed to save provider"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteLLMProvider(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: llmKeys.providers });
      qc.invalidateQueries({ queryKey: llmKeys.models });
      toast.success("Provider deleted");
    },
    onError: () => toast.error("Failed to delete provider"),
  });

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Configure LLM providers and their model catalogs. When an agent selects a model registered here, API credentials are automatically injected at runtime.
      </p>

      {providersQuery.isLoading && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />Loading providers...
        </div>
      )}

      {providersQuery.data?.map((provider) => (
        <Card key={provider.id}>
          <CardContent className="space-y-3 pt-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Cpu className="h-4 w-4 text-muted-foreground" />
                <span className="font-semibold">{provider.name}</span>
                <span className="text-xs text-muted-foreground font-mono">{provider.api_base_url}</span>
              </div>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="icon-sm" onClick={() => openEdit(provider)} title="Edit provider">
                  <Edit className="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon-sm" onClick={() => deleteMutation.mutate(provider.id)} title="Delete provider">
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            </div>
            <div className="space-y-1 pl-6">
              {modelsByProvider(provider.id).map((model) => (
                <div key={model.id} className="flex items-center gap-2 text-sm">
                  <span className="font-mono text-xs">{model.model_code}</span>
                  {model.name && (
                    <span className="text-muted-foreground">({model.name})</span>
                  )}
                  {model.capabilities?.length > 0 && (
                    <span className="text-xs text-muted-foreground">
                      {model.capabilities.join(", ")}
                    </span>
                  )}
                </div>
              ))}
              {modelsByProvider(provider.id).length === 0 && (
                <p className="text-xs text-muted-foreground italic">No models defined</p>
              )}
            </div>
          </CardContent>
        </Card>
      ))}

      {!providersQuery.isLoading && !providersQuery.data?.length && (
        <p className="text-sm text-muted-foreground italic">No providers configured yet.</p>
      )}

      <Button variant="outline" size="sm" onClick={openCreate}>
        <Plus className="h-4 w-4 mr-1" />
        Add Provider
      </Button>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="w-[60vw] max-h-[60vh] overflow-y-auto">
          <DialogTitle>{editing ? "Edit Provider" : "Add Provider"}</DialogTitle>
          <div className="space-y-3">
            <div>
              <label className="text-xs font-medium">Name</label>
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="e.g. DeepSeek" />
            </div>
            <div>
              <label className="text-xs font-medium">API Base URL</label>
              <Input value={form.api_base_url} onChange={(e) => setForm({ ...form, api_base_url: e.target.value })} placeholder="https://api.deepseek.com/anthropic" />
            </div>
            <div>
              <label className="text-xs font-medium">API Key</label>
              <Input type="password" value={form.api_key} onChange={(e) => setForm({ ...form, api_key: e.target.value })} placeholder="sk-..." />
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
            <Button variant="outline" size="sm" onClick={() => setDialogOpen(false)}>Cancel</Button>
            <Button size="sm" onClick={() => saveMutation.mutate()} disabled={!form.name || !form.api_key || saveMutation.isPending}>
              {saveMutation.isPending && <Loader2 className="h-3 w-3 animate-spin mr-1" />}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
