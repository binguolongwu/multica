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
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@multica/ui/components/ui/sheet";
import { Label } from "@multica/ui/components/ui/label";
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
  const [icon, setIcon] = useState(initial?.icon ?? "");
  const [accent, setAccent] = useState(initial?.accent ?? "");
  const [tagsInput, setTagsInput] = useState((initial?.tags ?? []).join(", "));
  const [instructions, setInstructions] = useState(initial?.instructions ?? "");
  const [model, setModel] = useState(initial?.model ?? "");
  const [saving, setSaving] = useState(false);

  const handleSubmit = async () => {
    setSaving(true);
    try {
      const tags = tagsInput
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
      await onSave({
        name,
        description,
        category,
        icon,
        accent,
        tags,
        instructions,
        model,
      });
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
          <Label className="text-sm font-medium">Icon (lucide name)</Label>
          <Input value={icon} onChange={(e) => setIcon(e.target.value)} placeholder="e.g. Code2" />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label className="text-sm font-medium">Accent</Label>
          <Input value={accent} onChange={(e) => setAccent(e.target.value)} placeholder="info/success/warning/primary" />
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

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center h-64 text-muted-foreground">
        Platform admin access required.
      </div>
    );
  }

  const filtered = templates.filter(
    (t) =>
      !search ||
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.category.toLowerCase().includes(search.toLowerCase()) ||
      t.tags.some((tag) => tag.toLowerCase().includes(search.toLowerCase())),
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Agent Template Library</h2>
          <p className="text-sm text-muted-foreground">
            Manage platform-level agent templates available to all workspaces.
          </p>
        </div>
        <Button onClick={() => setCreating(true)}>
          <Plus className="h-4 w-4 mr-1" /> New Template
        </Button>
      </div>

      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          className="pl-9"
          placeholder="Search templates..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {isLoading ? (
        <div className="text-center text-muted-foreground py-12">Loading...</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((t) => (
            <Card key={t.id} className="hover:shadow-md transition-shadow">
              <CardContent className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold truncate">{t.name}</h3>
                  <div className="flex gap-1">
                    <Button variant="ghost" size="icon" onClick={() => setEditing(t)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={async () => {
                        if (confirm(`Delete template "${t.name}"?`)) {
                          try {
                            await deleteMutation.mutateAsync(t.id);
                            toast.success("Template deleted");
                          } catch (err) {
                            toast.error(err instanceof Error ? err.message : "Delete failed");
                          }
                        }
                      }}
                    >
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </div>
                <p className="text-sm text-muted-foreground line-clamp-2">
                  {t.description}
                </p>
                <div className="flex flex-wrap gap-1">
                  {t.category && <Badge variant="secondary">{t.category}</Badge>}
                  {t.tags.map((tag) => (
                    <Badge key={tag} variant="outline" className="text-xs">
                      {tag}
                    </Badge>
                  ))}
                </div>
              </CardContent>
            </Card>
          ))}
          {filtered.length === 0 && !isLoading && (
            <div className="col-span-full text-center text-muted-foreground py-12">
              No templates found.
            </div>
          )}
        </div>
      )}

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
