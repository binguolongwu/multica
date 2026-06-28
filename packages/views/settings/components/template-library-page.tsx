"use client";

import { useState } from "react";
import { Plus, Pencil, Trash2, Search } from "lucide-react";
import {
  useAgentTemplates,
  useCreateAgentTemplate,
  useUpdateAgentTemplate,
  useDeleteAgentTemplate,
  usePlatformAdmin,
} from "@multica/core/agents/queries";
import type { AgentTemplate, CreateAgentTemplateRequest, UpdateAgentTemplateRequest } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Badge } from "@multica/ui/components/ui/badge";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@multica/ui/components/ui/sheet";
import { Label } from "@multica/ui/components/ui/label";
import { DataTable } from "@multica/ui/components/ui/data-table";
import { toast } from "sonner";

function TemplateEditForm({
  initial,
  onSave,
  onCancel,
  isCreating,
}: {
  initial?: AgentTemplate;
  onSave: (data: CreateAgentTemplateRequest | UpdateAgentTemplateRequest) => Promise<void>;
  onCancel: () => void;
  isCreating: boolean;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [category, setCategory] = useState(initial?.category ?? "");
  const [tagsInput, setTagsInput] = useState((initial?.tags ?? []).join(", "));
  const [instructions, setInstructions] = useState(initial?.instructions ?? "");
  const [model, setModel] = useState(initial?.model ?? "");
  const [saving, setSaving] = useState(false);

  const handleSubmit = async () => {
    setSaving(true);
    try {
      const tags = tagsInput.split(",").map((t) => t.trim()).filter(Boolean);
      await onSave({ name, description, category, tags, instructions, model });
      toast.success(isCreating ? "Template created" : "Template updated");
      onCancel();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4 p-4">
      <div>
        <Label className="text-sm font-medium">Name</Label>
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Template name" />
      </div>
      <div>
        <Label className="text-sm font-medium">Description</Label>
        <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Brief description" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label className="text-sm font-medium">Category</Label>
          <Input value={category} onChange={(e) => setCategory(e.target.value)} placeholder="e.g. Engineering" />
        </div>
        <div>
          <Label className="text-sm font-medium">Model</Label>
          <Input value={model} onChange={(e) => setModel(e.target.value)} placeholder="claude-sonnet-4-5" />
        </div>
      </div>
      <div>
        <Label className="text-sm font-medium">Tags (comma-separated)</Label>
        <Input value={tagsInput} onChange={(e) => setTagsInput(e.target.value)} placeholder="backend, api, go" />
      </div>
      <div>
        <Label className="text-sm font-medium">Instructions (markdown)</Label>
        <textarea
          className="w-full min-h-[200px] rounded-md border p-3 font-mono text-sm bg-background"
          value={instructions}
          onChange={(e) => setInstructions(e.target.value)}
          placeholder="Agent instructions..."
        />
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onCancel}>Cancel</Button>
        <Button onClick={handleSubmit} disabled={saving || !name.trim()}>
          {saving ? "Saving..." : "Save"}
        </Button>
      </div>
    </div>
  );
}

export function TemplateLibraryPage() {
  const { data: templates = [], isLoading } = useAgentTemplates();
  const { data: isAdmin } = usePlatformAdmin();
  const createMutation = useCreateAgentTemplate();
  const updateMutation = useUpdateAgentTemplate();
  const deleteMutation = useDeleteAgentTemplate();

  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<AgentTemplate | null>(null);
  const [creating, setCreating] = useState(false);

  const filtered = templates.filter(
    (t) =>
      !search ||
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.category.toLowerCase().includes(search.toLowerCase()) ||
      t.tags.some((tag) => tag.toLowerCase().includes(search.toLowerCase())),
  );

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Header */}
      <div className="flex items-center justify-between shrink-0 px-5 py-3 border-b">
        <div>
          <h1 className="text-sm font-semibold">Agent Templates</h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            {filtered.length} template{filtered.length !== 1 ? "s" : ""}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              className="pl-9 w-64"
              placeholder="Search templates..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          {isAdmin && (
            <Button onClick={() => setCreating(true)}>
              <Plus className="h-4 w-4 mr-1" /> New Template
            </Button>
          )}
        </div>
      </div>

      {/* Table */}
      <div className="flex-1 min-h-0 overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/50 text-muted-foreground text-xs">
              <th className="text-left font-medium px-5 py-2 w-[200px]">Name</th>
              <th className="text-left font-medium px-3 py-2">Description</th>
              <th className="text-left font-medium px-3 py-2 w-[100px]">Category</th>
              <th className="text-left font-medium px-3 py-2 w-[200px]">Tags</th>
              <th className="text-left font-medium px-3 py-2 w-[60px]">Skills</th>
              <th className="text-left font-medium px-3 py-2 w-[120px]">Created</th>
              <th className="text-right font-medium px-5 py-2 w-[80px]">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="border-b animate-pulse">
                  {Array.from({ length: 7 }).map((_, j) => (
                    <td key={j} className="px-3 py-3">
                      <div className="h-4 bg-muted rounded w-3/4" />
                    </td>
                  ))}
                </tr>
              ))
            )}
            {!isLoading && filtered.map((row) => (
              <tr key={row.id} className="border-b hover:bg-muted/50 transition-colors">
                <td className="px-5 py-3 font-medium truncate max-w-[200px]">{row.name}</td>
                <td className="px-3 py-3 text-muted-foreground truncate max-w-[300px]">{row.description}</td>
                <td className="px-3 py-3">
                  {row.category && <Badge variant="secondary" className="text-xs">{row.category}</Badge>}
                </td>
                <td className="px-3 py-3">
                  <div className="flex flex-wrap gap-1">
                    {row.tags.slice(0, 3).map((tag) => (
                      <Badge key={tag} variant="outline" className="text-xs">{tag}</Badge>
                    ))}
                    {row.tags.length > 3 && (
                      <span className="text-xs text-muted-foreground">+{row.tags.length - 3}</span>
                    )}
                  </div>
                </td>
                <td className="px-3 py-3 text-muted-foreground">{row.skill_urls.length}</td>
                <td className="px-3 py-3 text-muted-foreground text-xs">
                  {new Date(row.created_at).toLocaleDateString()}
                </td>
                <td className="px-5 py-3">
                  {isAdmin && (
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon-sm" onClick={() => setEditing(row)}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={async () => {
                          if (confirm('Delete template "' + row.name + '"?')) {
                            try {
                              await deleteMutation.mutateAsync(row.id);
                              toast.success("Template deleted");
                            } catch (err) {
                              toast.error(err instanceof Error ? err.message : "Delete failed");
                            }
                          }
                        }}
                      >
                        <Trash2 className="h-3.5 w-3.5 text-destructive" />
                      </Button>
                    </div>
                  )}
                </td>
              </tr>
            ))}
            {!isLoading && filtered.length === 0 && (
              <tr>
                <td colSpan={7} className="text-center text-muted-foreground py-12">
                  No templates found.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Create sheet */}
      <Sheet open={creating} onOpenChange={(v) => { if (!v) setCreating(false); }}>
        <SheetContent side="right" className="w-full max-w-xl p-0">
          <SheetHeader className="border-b px-4 py-3">
            <SheetTitle>Create Template</SheetTitle>
          </SheetHeader>
          <TemplateEditForm
            isCreating
            onSave={async (data) => {
              await createMutation.mutateAsync(data as CreateAgentTemplateRequest);
            }}
            onCancel={() => setCreating(false)}
          />
        </SheetContent>
      </Sheet>

      {/* Edit sheet */}
      <Sheet open={!!editing} onOpenChange={(v) => { if (!v) setEditing(null); }}>
        <SheetContent side="right" className="w-full max-w-xl p-0">
          <SheetHeader className="border-b px-4 py-3">
            <SheetTitle>Edit Template</SheetTitle>
          </SheetHeader>
          {editing && (
            <TemplateEditForm
              isCreating={false}
              initial={editing}
              onSave={async (data) => {
                await updateMutation.mutateAsync({
                  id: editing.id,
                  data: data as UpdateAgentTemplateRequest,
                });
              }}
              onCancel={() => setEditing(null)}
            />
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
