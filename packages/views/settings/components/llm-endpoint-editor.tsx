"use client";

import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { LLMProviderEndpoint, APIType, CreateEndpointRequest } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";

const API_TYPES: { value: APIType; label: string }[] = [
  { value: "anthropic", label: "Anthropic" },
  { value: "openai_chat", label: "OpenAI Chat Completions" },
  { value: "openai_responses", label: "OpenAI Responses API" },
];

export function LLMEndpointEditor({
  workspaceId,
  providerId,
  endpoints,
}: {
  workspaceId: string;
  providerId: string;
  endpoints: LLMProviderEndpoint[];
}) {
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [newType, setNewType] = useState<APIType>("anthropic");
  const [newUrl, setNewUrl] = useState("");

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["llm-providers", workspaceId] });
  };

  const handleAdd = async () => {
    if (!newUrl.trim()) return;
    try {
      await api.createProviderEndpoint(workspaceId, providerId, {
        api_type: newType,
        api_base_url: newUrl.trim(),
      });
      setNewUrl("");
      setAdding(false);
      invalidate();
      toast.success("端点已添加");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "添加端点失败");
    }
  };

  const handleDelete = async (endpointId: string) => {
    try {
      await api.deleteProviderEndpoint(workspaceId, providerId, endpointId);
      invalidate();
      toast.success("端点已删除");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除端点失败");
    }
  };

  const handleUpdateUrl = async (endpointId: string, url: string) => {
    try {
      await api.updateProviderEndpoint(workspaceId, providerId, endpointId, {
        api_base_url: url,
      });
      invalidate();
    } catch {
      toast.error("更新端点失败");
    }
  };

  return (
    <div className="space-y-3">
      <Label className="text-xs text-muted-foreground">API 端点</Label>

      {endpoints.map((ep) => (
        <div key={ep.endpoint_id} className="flex items-center gap-2">
          <span className="min-w-[140px] rounded-md bg-muted px-2 py-1.5 text-xs font-medium">
            {API_TYPES.find((t) => t.value === ep.api_type)?.label ?? ep.api_type}
          </span>
          <Input
            className="h-8 flex-1 font-mono text-xs"
            defaultValue={ep.api_base_url}
            onBlur={(e) => {
              if (e.target.value !== ep.api_base_url) {
                handleUpdateUrl(ep.endpoint_id, e.target.value);
              }
            }}
          />
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => handleDelete(ep.endpoint_id)}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}

      {adding && (
        <div className="flex items-center gap-2">
          <select
            className="h-8 min-w-[140px] rounded-md border bg-transparent px-2 text-xs"
            value={newType}
            onChange={(e) => setNewType(e.target.value as APIType)}
          >
            {API_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
          <Input
            className="h-8 flex-1 font-mono text-xs"
            placeholder="https://api.example.com/v1"
            value={newUrl}
            onChange={(e) => setNewUrl(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleAdd(); }}
          />
          <Button size="sm" onClick={handleAdd} disabled={!newUrl.trim()}>
            添加
          </Button>
        </div>
      )}

      {!adding && (
        <Button
          variant="outline"
          size="sm"
          onClick={() => setAdding(true)}
        >
          <Plus className="h-3 w-3" />
          添加端点
        </Button>
      )}
    </div>
  );
}
