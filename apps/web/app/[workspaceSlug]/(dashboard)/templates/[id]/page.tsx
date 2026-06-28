"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, Trash2 } from "lucide-react";
import {
  useAgentTemplate,
  useUpdateAgentTemplate,
  useDeleteAgentTemplate,
  usePlatformAdmin,
} from "@multica/core/agents/queries";
import type { UpdateAgentTemplateRequest } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Badge } from "@multica/ui/components/ui/badge";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@multica/ui/components/ui/tabs";
import { toast } from "sonner";
import { useT } from "@multica/views/i18n";

export default function TemplateDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const p = useWorkspacePaths();
  const id = params.id;
  const { t } = useT("agents");

  const { data: template, isLoading } = useAgentTemplate(id);
  const { data: isAdmin } = usePlatformAdmin();
  const updateMutation = useUpdateAgentTemplate();
  const deleteMutation = useDeleteAgentTemplate();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [instructions, setInstructions] = useState("");
  const [skillUrlsInput, setSkillUrlsInput] = useState("");
  const [mcpConfigInput, setMcpConfigInput] = useState("");
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (template) {
      setName(template.name);
      setDescription(template.description);
      setCategory(template.category);
      setTagsInput((template.tags ?? []).join(", "));
      setInstructions(template.instructions);
      setSkillUrlsInput((template.skill_urls ?? []).join("\n"));
      setMcpConfigInput(template.mcp_config ? JSON.stringify(template.mcp_config, null, 2) : "");
      setDirty(false);
    }
  }, [template]);

  const markDirty = () => setDirty(true);

  const buildUpdate = (): UpdateAgentTemplateRequest => {
    if (!template) return {};
    const data: UpdateAgentTemplateRequest = {};
    if (name !== template.name) data.name = name;
    if (description !== template.description) data.description = description;
    if (category !== template.category) data.category = category;
    if (instructions !== template.instructions) data.instructions = instructions;

    const newTags = tagsInput.split(",").map((x) => x.trim()).filter(Boolean);
    const oldTags = template.tags ?? [];
    if (newTags.join(",") !== oldTags.join(",")) data.tags = newTags;

    const newSkillUrls = skillUrlsInput.split("\n").map((x) => x.trim()).filter(Boolean);
    const oldSkillUrls = template.skill_urls ?? [];
    if (newSkillUrls.join(",") !== oldSkillUrls.join(",")) data.skill_urls = newSkillUrls;

    if (mcpConfigInput.trim()) {
      try {
        const parsed = JSON.parse(mcpConfigInput);
        data.mcp_config = parsed;
      } catch { /* skip invalid */ }
    } else if (template.mcp_config) {
      data.mcp_config = null as unknown as undefined;
    }

    return data;
  };

  const handleSave = async () => {
    if (!template) return;
    setSaving(true);
    try {
      await updateMutation.mutateAsync({ id: template.id, data: buildUpdate() });
      toast.success(t(($) => $.template_editor.updated));
      setDirty(false);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.template_editor.update_failed));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!template) return;
    if (!confirm(t(($) => $.template_editor.delete_confirm, { name: template.name }))) return;
    try {
      await deleteMutation.mutateAsync(template.id);
      toast.success(t(($) => $.template_editor.deleted));
      router.push(p.templates());
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.template_editor.delete_failed));
    }
  };

  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0 gap-4 p-6">
        <Skeleton className="w-80 h-full rounded-lg" />
        <Skeleton className="flex-1 h-full rounded-lg" />
      </div>
    );
  }

  if (!template) {
    return (
      <div className="flex items-center justify-center flex-1 text-muted-foreground">
        {t(($) => $.template_editor.not_found)}
      </div>
    );
  }

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Header */}
      <div className="flex items-center justify-between shrink-0 px-5 py-3 border-b">
        <div className="flex items-center gap-3 min-w-0">
          <Button variant="ghost" size="icon-sm" onClick={() => router.push(p.templates())}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="min-w-0">
            <h1 className="text-sm font-semibold truncate">{template.name}</h1>
            <p className="text-xs text-muted-foreground">{t(($) => $.template_editor.template)}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {dirty && (
            <Button size="sm" onClick={handleSave} disabled={saving}>
              {saving ? t(($) => $.template_editor.saving) : t(($) => $.template_editor.save)}
            </Button>
          )}
          {isAdmin && (
            <Button variant="ghost" size="icon-sm" onClick={handleDelete}>
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          )}
        </div>
      </div>

      <div className="flex flex-1 min-h-0 flex-col gap-3 overflow-y-auto p-3 md:grid md:grid-cols-[320px_minmax(0,1fr)] md:gap-4 md:overflow-hidden md:p-6">

        {/* Left sidebar */}
        <aside className="flex w-full flex-col rounded-lg border bg-background md:h-full md:min-h-0 md:overflow-y-auto">

          {/* Identity */}
          <div className="flex flex-col gap-3 border-b px-5 pb-5 pt-5">
            <div className="flex flex-col gap-1">
              <input
                className="w-full bg-transparent text-base font-semibold leading-tight outline-none"
                value={name}
                onChange={(e) => { setName(e.target.value); markDirty(); }}
                placeholder="Template name"
              />
              <input
                className="w-full bg-transparent text-xs leading-relaxed text-muted-foreground outline-none"
                value={description}
                onChange={(e) => { setDescription(e.target.value); markDirty(); }}
                placeholder="Description"
              />
            </div>
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="inline-flex items-center gap-1.5 rounded-md border px-1.5 py-0.5 text-xs text-muted-foreground">
                <span className="h-1.5 w-1.5 rounded-full bg-green-400" />
                {t(($) => $.template_editor.active)}
              </span>
            </div>
          </div>

          {/* Properties */}
          <div className="border-b px-5 py-4">
            <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t(($) => $.template_editor.properties)}
            </div>
            <div className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.category)}</span>
                <Input className="h-7 text-xs" value={category}
                  onChange={(e) => { setCategory(e.target.value); markDirty(); }}
                  placeholder="e.g. Engineering" />
              </div>
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.visibility)}</span>
                <span className="text-xs">{template.visibility}</span>
              </div>
            </div>
          </div>

          {/* Tags */}
          <div className="border-b px-5 py-4">
            <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t(($) => $.template_editor.tags)}
            </div>
            <Input className="h-7 text-xs mt-1" value={tagsInput}
              onChange={(e) => { setTagsInput(e.target.value); markDirty(); }}
              placeholder="backend, api, go" />
            {tagsInput && (
              <div className="flex flex-wrap gap-1 mt-2">
                {tagsInput.split(",").map((x) => x.trim()).filter(Boolean).map((tag) => (
                  <Badge key={tag} variant="outline" className="text-xs">{tag}</Badge>
                ))}
              </div>
            )}
          </div>

          {/* Details */}
          <div className="px-5 py-4">
            <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t(($) => $.template_editor.details)}
            </div>
            <div className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.skills)}</span>
                <span className="text-xs">{(template.skill_urls ?? []).length}</span>
              </div>
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.created)}</span>
                <span className="text-xs">{new Date(template.created_at).toLocaleDateString()}</span>
              </div>
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.updated)}</span>
                <span className="text-xs">{new Date(template.updated_at).toLocaleDateString()}</span>
              </div>
            </div>
          </div>
        </aside>

        {/* Right content -- Tabs */}
        <div className="flex flex-1 flex-col min-h-0">
          <Tabs defaultValue="instructions" className="flex flex-1 flex-col min-h-0">
            <TabsList className="w-fit shrink-0">
              <TabsTrigger value="instructions">Instructions</TabsTrigger>
              <TabsTrigger value="skills">Skills</TabsTrigger>
              <TabsTrigger value="mcp">MCP</TabsTrigger>
            </TabsList>

            <TabsContent value="instructions" className="flex-1 min-h-0 mt-3 data-[state=active]:flex data-[state=active]:flex-col">
              <Label className="text-xs text-muted-foreground mb-2">
                Define the agent&apos;s identity and working style. Injected into every task context. Supports Markdown.
              </Label>
              <textarea
                className="flex-1 min-h-[400px] w-full rounded-md border p-4 font-mono text-sm bg-background resize-none focus:outline-none focus:ring-2 focus:ring-ring/50"
                value={instructions}
                onChange={(e) => { setInstructions(e.target.value); markDirty(); }}
                placeholder="Agent instructions (markdown)..." />
            </TabsContent>

            <TabsContent value="skills" className="flex-1 min-h-0 mt-3 data-[state=active]:flex data-[state=active]:flex-col">
              <Label className="text-xs text-muted-foreground mb-2">
                External skill URLs (one per line). Imported when an agent is created from this template.
              </Label>
              <textarea
                className="flex-1 min-h-[400px] w-full rounded-md border p-4 font-mono text-sm bg-background resize-none focus:outline-none focus:ring-2 focus:ring-ring/50"
                value={skillUrlsInput}
                onChange={(e) => { setSkillUrlsInput(e.target.value); markDirty(); }}
                placeholder="https://github.com/vercel-labs/agent-skills/tree/main/skills/react-best-practices" />
            </TabsContent>

            <TabsContent value="mcp" className="flex-1 min-h-0 mt-3 data-[state=active]:flex data-[state=active]:flex-col">
              <Label className="text-xs text-muted-foreground mb-2">
                MCP server configuration (JSON format).
              </Label>
              <textarea
                className="flex-1 min-h-[400px] w-full rounded-md border p-4 font-mono text-sm bg-background resize-none focus:outline-none focus:ring-2 focus:ring-ring/50"
                value={mcpConfigInput}
                onChange={(e) => { setMcpConfigInput(e.target.value); markDirty(); }}
                placeholder='{"servers": {}}' />
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
