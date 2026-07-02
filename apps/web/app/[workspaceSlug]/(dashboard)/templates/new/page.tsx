"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { useCreateAgentTemplate } from "@multica/core/agents/queries";
import type { CreateAgentTemplateRequest } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { toast } from "sonner";

export default function NewTemplatePage() {
  const router = useRouter();
  const p = useWorkspacePaths();
  const createMutation = useCreateAgentTemplate();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [instructions, setInstructions] = useState("");
  const [model, setModel] = useState("");
  const [saving, setSaving] = useState(false);

  const handleCreate = async () => {
    if (!name.trim()) return;
    setSaving(true);
    try {
      const tags = tagsInput.split(",").map((t) => t.trim()).filter(Boolean);
      const data: CreateAgentTemplateRequest = { name: name.trim() };
      if (description) data.description = description;
      if (category) data.category = category;
      if (tags.length) data.tags = tags;
      if (instructions) data.instructions = instructions;
      if (model) data.model = model;

      const created = await createMutation.mutateAsync(data);
      toast.success("模板已创建");
      router.push(p.templates() + "/" + created.id);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "创建失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Header */}
      <div className="flex items-center justify-between shrink-0 px-5 py-3 border-b">
        <div className="flex items-center gap-3 min-w-0">
          <Button variant="ghost" size="icon-sm" onClick={() => router.push(p.templates())}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-sm font-semibold">新建模板</h1>
            <p className="text-xs text-muted-foreground">创建新的 Agent 模板</p>
          </div>
        </div>
        <Button size="sm" onClick={handleCreate} disabled={saving || !name.trim()}>
          {saving ? "创建中..." : "创建模板"}
        </Button>
      </div>

      {/* Form */}
      <div className="flex-1 min-h-0 overflow-y-auto p-6 max-w-2xl">
        <div className="space-y-5">
          <div>
            <Label className="text-xs font-medium text-muted-foreground">名称 *</Label>
            <Input
              className="mt-1"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="模板名称"
              autoFocus
            />
          </div>
          <div>
            <Label className="text-xs font-medium text-muted-foreground">描述</Label>
            <Input
              className="mt-1"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="简要描述"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs font-medium text-muted-foreground">分类</Label>
              <Input
                className="mt-1"
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                placeholder="如：工程"
              />
            </div>
            <div>
              <Label className="text-xs font-medium text-muted-foreground">模型</Label>
              <Input
                className="mt-1"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="claude-sonnet-4-5"
              />
            </div>
          </div>
          <div>
            <Label className="text-xs font-medium text-muted-foreground">标签（逗号分隔）</Label>
            <Input
              className="mt-1"
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
              placeholder="backend, api, go"
            />
          </div>
          <div>
            <Label className="text-xs font-medium text-muted-foreground">指令</Label>
            <textarea
              className="w-full min-h-[300px] mt-1 rounded-md border p-3 font-mono text-sm bg-background resize-y"
              value={instructions}
              onChange={(e) => setInstructions(e.target.value)}
              placeholder="Agent 指令（Markdown）..."
            />
          </div>
        </div>
      </div>
    </div>
  );
}
