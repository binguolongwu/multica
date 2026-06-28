"use client";

import { useState, useEffect, useCallback } from "react";
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
import { toast } from "sonner";

export default function TemplateDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const p = useWorkspacePaths();
  const id = params.id;

  const { data: template, isLoading } = useAgentTemplate(id);
  const { data: isAdmin } = usePlatformAdmin();
  const updateMutation = useUpdateAgentTemplate();
  const deleteMutation = useDeleteAgentTemplate();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [instructions, setInstructions] = useState("");
  const [model, setModel] = useState("");
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (template) {
      setName(template.name);
      setDescription(template.description);
      setCategory(template.category);
      setTagsInput((template.tags ?? []).join(", "));
      setInstructions(template.instructions);
      setModel(template.model);
      setDirty(false);
    }
  }, [template]);

  const markDirty = () => setDirty(true);

  const handleSave = useCallback(async () => {
    if (!template) return;
    setSaving(true);
    try {
      const tags = tagsInput.split(",").map((t) => t.trim()).filter(Boolean);
      const data: UpdateAgentTemplateRequest = {};
      if (name !== template.name) data.name = name;
      if (description !== template.description) data.description = description;
      if (category !== template.category) data.category = category;
      if (instructions !== template.instructions) data.instructions = instructions;
      if (model !== template.model) data.model = model;
      if (tagsInput !== (template.tags ?? []).join(", ")) data.tags = tags;

      await updateMutation.mutateAsync({ id: template.id, data });
      toast.success("Template updated");
      setDirty(false);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Update failed");
    } finally {
      setSaving(false);
    }
  }, [template, name, description, category, tagsInput, instructions, model, updateMutation]);

  const handleDelete = async () => {
    if (!template || !confirm(`Delete template "${template.name}"?`)) return;
    try {
      await deleteMutation.mutateAsync(template.id);
      toast.success("Template deleted");
      router.push(p.templates());
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Delete failed");
    }
  };

  if (isLoading) {
    return (
      <div className="flex flex-col flex-1 min-h-0 gap-4 p-6">
        <Skeleton className="h-6 w-48" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }

  if (!template) {
    return (
      <div className="flex items-center justify-center flex-1 text-muted-foreground">
        Template not found.
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
            <p className="text-xs text-muted-foreground">Template</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {dirty && (
            <Button size="sm" onClick={handleSave} disabled={saving}>
              {saving ? "Saving..." : "Save Changes"}
            </Button>
          )}
          {isAdmin && (
            <Button variant="ghost" size="icon-sm" onClick={handleDelete}>
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          )}
        </div>
      </div>

      {/* Body — two column layout */}
      <div className="flex-1 min-h-0 overflow-y-auto p-6 md:grid md:grid-cols-[320px_minmax(0,1fr)] md:gap-6 md:overflow-hidden">
        {/* Left: Edit form */}
        <div className="space-y-5 md:overflow-y-auto md:pr-2">
          <div>
            <Label className="text-xs font-medium text-muted-foreground">Name</Label>
            <Input
              className="mt-1"
              value={name}
              onChange={(e) => { setName(e.target.value); markDirty(); }}
            />
          </div>
          <div>
            <Label className="text-xs font-medium text-muted-foreground">Description</Label>
            <Input
              className="mt-1"
              value={description}
              onChange={(e) => { setDescription(e.target.value); markDirty(); }}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs font-medium text-muted-foreground">Category</Label>
              <Input
                className="mt-1"
                value={category}
                onChange={(e) => { setCategory(e.target.value); markDirty(); }}
                placeholder="e.g. Engineering"
              />
            </div>
            <div>
              <Label className="text-xs font-medium text-muted-foreground">Model</Label>
              <Input
                className="mt-1"
                value={model}
                onChange={(e) => { setModel(e.target.value); markDirty(); }}
                placeholder="claude-sonnet-4-5"
              />
            </div>
          </div>
          <div>
            <Label className="text-xs font-medium text-muted-foreground">Tags (comma-separated)</Label>
            <Input
              className="mt-1"
              value={tagsInput}
              onChange={(e) => { setTagsInput(e.target.value); markDirty(); }}
              placeholder="backend, api, go"
            />
          </div>
          <div>
            <Label className="text-xs font-medium text-muted-foreground">Instructions</Label>
            <textarea
              className="w-full min-h-[300px] mt-1 rounded-md border p-3 font-mono text-sm bg-background resize-y"
              value={instructions}
              onChange={(e) => { setInstructions(e.target.value); markDirty(); }}
              placeholder="Agent instructions (markdown)..."
            />
          </div>
        </div>

        {/* Right: Info / preview */}
        <div className="space-y-4 md:overflow-y-auto md:pl-2 mt-6 md:mt-0">
          <div className="rounded-lg border p-4 space-y-3">
            <h3 className="text-sm font-medium">Template Info</h3>
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <span className="text-xs text-muted-foreground">Skills</span>
                <p className="font-medium">{template.skill_urls.length}</p>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Created</span>
                <p className="font-medium">{new Date(template.created_at).toLocaleDateString()}</p>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Updated</span>
                <p className="font-medium">{new Date(template.updated_at).toLocaleDateString()}</p>
              </div>
              <div>
                <span className="text-xs text-muted-foreground">Visibility</span>
                <p className="font-medium">{template.visibility}</p>
              </div>
            </div>
          </div>

          {template.tags.length > 0 && (
            <div className="rounded-lg border p-4 space-y-2">
              <h3 className="text-xs font-medium text-muted-foreground">Tags</h3>
              <div className="flex flex-wrap gap-1">
                {template.tags.map((tag) => (
                  <Badge key={tag} variant="outline" className="text-xs">{tag}</Badge>
                ))}
              </div>
            </div>
          )}

          {template.skill_urls.length > 0 && (
            <div className="rounded-lg border p-4 space-y-2">
              <h3 className="text-xs font-medium text-muted-foreground">Skill URLs</h3>
              <ul className="space-y-1">
                {template.skill_urls.map((url, i) => (
                  <li key={i} className="text-xs text-muted-foreground truncate">{url}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
