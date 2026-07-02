"use client";

import { useState, useEffect, useMemo } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, Trash2, Plus, FileText, Search } from "lucide-react";
import {
  useAgentTemplate,
  useUpdateAgentTemplate,
  useDeleteAgentTemplate,
  usePlatformAdmin,
} from "@multica/core/agents/queries";
import type { UpdateAgentTemplateRequest } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { SkillSummary } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Badge } from "@multica/ui/components/ui/badge";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@multica/ui/components/ui/tabs";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@multica/ui/components/ui/dialog";
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

  // Left sidebar state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [sidebarDirty, setSidebarDirty] = useState(false);

  // Tab state
  const [instructions, setInstructions] = useState("");
  const [instructionsDraft, setInstructionsDraft] = useState("");
  const [skillIds, setSkillIds] = useState<string[]>([]);
  const [skillIdsDraft, setSkillIdsDraft] = useState<string[]>([]);
  const [showSkillPicker, setShowSkillPicker] = useState(false);
  const [skillSearch, setSkillSearch] = useState("");
  const [mcpConfigInput, setMcpConfigInput] = useState("");
  const [mcpConfigDraft, setMcpConfigDraft] = useState("");

  const [saving, setSaving] = useState(false);

  // Fetch platform + builtin skills for the picker
  const { data: platformSkills = [] } = useQuery({
    queryKey: ["platform-skills"],
    queryFn: () => api.listPlatformSkills(),
    staleTime: 30_000,
  });

  const selectedSkills = useMemo(() =>
    platformSkills.filter((s: SkillSummary) => skillIdsDraft.includes(s.id)),
    [platformSkills, skillIdsDraft]
  );

  const availableSkills = useMemo(() =>
    platformSkills.filter((s: SkillSummary) =>
      !skillIdsDraft.includes(s.id) &&
      (skillSearch === "" || s.name.toLowerCase().includes(skillSearch.toLowerCase()))
    ),
    [platformSkills, skillIdsDraft, skillSearch]
  );

  useEffect(() => {
    if (template) {
      setName(template.name);
      setDescription(template.description);
      setCategory(template.category);
      setTagsInput((template.tags ?? []).join(", "));
      setInstructions(template.instructions);
      setInstructionsDraft(template.instructions);
      const ids = template.skill_ids ?? [];
      setSkillIds(ids);
      setSkillIdsDraft([...ids]);
      const mcp = template.mcp_config ? JSON.stringify(template.mcp_config, null, 2) : "";
      setMcpConfigInput(mcp);
      setMcpConfigDraft(mcp);
      setSidebarDirty(false);
    }
  }, [template]);

  const instructionsDirty = instructionsDraft !== instructions;
  const skillsDirty = JSON.stringify(skillIdsDraft) !== JSON.stringify(skillIds);
  const mcpDirty = mcpConfigDraft !== mcpConfigInput;

  const doSave = async (data: UpdateAgentTemplateRequest) => {
    if (!template) return false;
    setSaving(true);
    try {
      await updateMutation.mutateAsync({ id: template.id, data });
      return true;
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.template_editor.update_failed));
      return false;
    } finally {
      setSaving(false);
    }
  };

  const handleSaveSidebar = async () => {
    const data: UpdateAgentTemplateRequest = {};
    if (name !== template!.name) data.name = name;
    if (description !== template!.description) data.description = description;
    if (category !== template!.category) data.category = category;
    const newTags = tagsInput.split(",").map((x) => x.trim()).filter(Boolean);
    const oldTags = template!.tags ?? [];
    if (newTags.join(",") !== oldTags.join(",")) data.tags = newTags;
    if (Object.keys(data).length === 0) return;
    if (await doSave(data)) { setSidebarDirty(false); toast.success(t(($) => $.template_editor.updated)); }
  };

  const handleSaveInstructions = async () => {
    if (await doSave({ instructions: instructionsDraft })) {
      setInstructions(instructionsDraft); toast.success(t(($) => $.template_editor.updated));
    }
  };

  const handleSaveSkills = async () => {
    if (await doSave({ skill_ids: skillIdsDraft })) {
      setSkillIds([...skillIdsDraft]); toast.success(t(($) => $.template_editor.updated));
    }
  };

  const handleSaveMcp = async () => {
    const data: UpdateAgentTemplateRequest = {};
    if (mcpConfigDraft.trim()) {
      try { data.mcp_config = JSON.parse(mcpConfigDraft); }
      catch { toast.error("Invalid JSON"); return; }
    } else { data.mcp_config = null as unknown as undefined; }
    if (await doSave(data)) { setMcpConfigInput(mcpConfigDraft); toast.success(t(($) => $.template_editor.updated)); }
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
          <div className="flex flex-col gap-3 border-b px-5 pb-5 pt-5">
            <div className="flex flex-col gap-1">
              <input className="w-full bg-transparent text-base font-semibold leading-tight outline-none"
                value={name} onChange={(e) => { setName(e.target.value); setSidebarDirty(true); }}
                placeholder="Template name" />
              <input className="w-full bg-transparent text-xs leading-relaxed text-muted-foreground outline-none"
                value={description} onChange={(e) => { setDescription(e.target.value); setSidebarDirty(true); }}
                placeholder="Description" />
            </div>
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="inline-flex items-center gap-1.5 rounded-md border px-1.5 py-0.5 text-xs text-muted-foreground">
                <span className="h-1.5 w-1.5 rounded-full bg-green-400" />
                {t(($) => $.template_editor.active)}
              </span>
            </div>
          </div>

          <div className="border-b px-5 py-4">
            <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t(($) => $.template_editor.properties)}
            </div>
            <div className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.category)}</span>
                <Input className="h-7 text-xs" value={category}
                  onChange={(e) => { setCategory(e.target.value); setSidebarDirty(true); }} placeholder="e.g. Engineering" />
              </div>
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.visibility)}</span>
                <span className="text-xs">{template.visibility}</span>
              </div>
            </div>
          </div>

          <div className="border-b px-5 py-4">
            <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t(($) => $.template_editor.tags)}
            </div>
            <Input className="h-7 text-xs mt-1" value={tagsInput}
              onChange={(e) => { setTagsInput(e.target.value); setSidebarDirty(true); }} placeholder="backend, api, go" />
            {tagsInput && (
              <div className="flex flex-wrap gap-1 mt-2">
                {tagsInput.split(",").map((x) => x.trim()).filter(Boolean).map((tag) => (
                  <Badge key={tag} variant="outline" className="text-xs">{tag}</Badge>
                ))}
              </div>
            )}
          </div>

          <div className="flex-1" />

          <div className="border-t px-5 py-4">
            <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t(($) => $.template_editor.details)}
            </div>
            <div className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
              <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
                <span className="text-xs text-muted-foreground">{t(($) => $.template_editor.skills)}</span>
                <span className="text-xs">{skillIdsDraft.length}</span>
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

          {sidebarDirty && (
            <div className="border-t px-5 py-3">
              <Button size="sm" className="w-full" onClick={handleSaveSidebar} disabled={saving}>
                {saving ? t(($) => $.template_editor.saving) : t(($) => $.template_editor.save)}
              </Button>
            </div>
          )}
        </aside>

        {/* Right content -- Tabs */}
        <div className="flex flex-1 flex-col min-h-0">
          <Tabs defaultValue="instructions" className="flex flex-1 flex-col min-h-0">
            <TabsList className="w-fit shrink-0">
              <TabsTrigger value="instructions">Instructions</TabsTrigger>
              <TabsTrigger value="skills">Skills</TabsTrigger>
              <TabsTrigger value="mcp">MCP</TabsTrigger>
            </TabsList>

            {/* Instructions tab */}
            <TabsContent value="instructions" className="flex-1 flex flex-col min-h-0 mt-3">
              <Label className="text-xs text-muted-foreground mb-2 shrink-0">
                Define the agent&apos;s identity and working style. Injected into every task context. Supports Markdown.
              </Label>
              <textarea
                className="flex-1 min-h-0 w-full rounded-md border p-4 font-mono text-sm bg-background resize-none focus:outline-none focus:ring-2 focus:ring-ring/50"
                value={instructionsDraft} onChange={(e) => setInstructionsDraft(e.target.value)}
                placeholder="Agent instructions (markdown)..." />
              {instructionsDirty && (
                <div className="shrink-0 mt-3">
                  <Button size="sm" onClick={handleSaveInstructions} disabled={saving}>
                    {saving ? t(($) => $.template_editor.saving) : t(($) => $.template_editor.save)}
                  </Button>
                </div>
              )}
            </TabsContent>

            {/* Skills tab */}
            <TabsContent value="skills" className="flex-1 flex flex-col min-h-0 mt-3">
              <div className="flex h-full flex-col p-4 md:p-6">
                <div className="space-y-4">
                  <div className="flex items-center justify-between gap-3">
                    <p className="text-xs text-muted-foreground">
                      Select platform and built-in skills for this template.
                    </p>
                    <Button size="sm" variant="outline" className="shrink-0" onClick={() => setShowSkillPicker(true)}>
                      <Plus className="h-3 w-3" /> Add Skill
                    </Button>
                  </div>

                  {/* Selected skills list */}
                  {selectedSkills.length > 0 && (
                    <ul className="space-y-1.5">
                      {selectedSkills.map((skill) => (
                        <li key={skill.id} className="flex items-center gap-2.5 rounded-md border px-3 py-2">
                          <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="text-sm font-medium truncate">{skill.name}</div>
                            {skill.description && (
                              <div className="truncate text-xs text-muted-foreground">{skill.description}</div>
                            )}
                          </div>
                          <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium shrink-0 ${
                            skill.is_builtin ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400" : "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                          }`}>
                            {skill.is_builtin ? "builtin" : "platform"}
                          </span>
                          <Button
                            variant="ghost" size="icon-sm"
                            className="text-muted-foreground hover:text-destructive shrink-0"
                            onClick={() => setSkillIdsDraft(prev => prev.filter(id => id !== skill.id))}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </li>
                      ))}
                    </ul>
                  )}

                  {selectedSkills.length === 0 && (
                    <p className="text-xs text-muted-foreground">No skills selected yet.</p>
                  )}
                </div>

                {skillsDirty && (
                  <div className="shrink-0 mt-auto pt-4">
                    <Button size="sm" onClick={handleSaveSkills} disabled={saving}>
                      {saving ? t(($) => $.template_editor.saving) : t(($) => $.template_editor.save)}
                    </Button>
                  </div>
                )}

                {/* Skill picker dialog */}
                <Dialog open={showSkillPicker} onOpenChange={setShowSkillPicker}>
                  <DialogContent className="max-w-md">
                    <DialogHeader>
                      <DialogTitle className="text-sm">Add Skills</DialogTitle>
                    </DialogHeader>
                    <div className="relative">
                      <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
                      <Input className="h-8 pl-8 text-xs" placeholder="Search skills..."
                        value={skillSearch} onChange={(e) => setSkillSearch(e.target.value)} />
                    </div>
                    <div className="max-h-64 overflow-y-auto space-y-1">
                      {availableSkills.map((skill) => (
                        <button
                          key={skill.id}
                          type="button"
                          onClick={() => { setSkillIdsDraft(prev => [...prev, skill.id]); setSkillSearch(""); }}
                          className="w-full text-left flex items-center gap-2.5 rounded-md border px-3 py-2 hover:bg-accent"
                        >
                          <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="text-sm font-medium truncate">{skill.name}</div>
                            {skill.description && (
                              <div className="truncate text-xs text-muted-foreground">{skill.description}</div>
                            )}
                          </div>
                          <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium shrink-0 ${
                            skill.is_builtin ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400" : "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                          }`}>
                            {skill.is_builtin ? "builtin" : "platform"}
                          </span>
                        </button>
                      ))}
                      {availableSkills.length === 0 && (
                        <p className="text-xs text-muted-foreground text-center py-4">
                          All available skills are already selected.
                        </p>
                      )}
                    </div>
                    <DialogFooter>
                      <Button variant="ghost" onClick={() => { setShowSkillPicker(false); setSkillSearch(""); }}>
                        Done
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              </div>
            </TabsContent>

            {/* MCP tab */}
            <TabsContent value="mcp" className="flex-1 flex flex-col min-h-0 mt-3">
              <Label className="text-xs text-muted-foreground mb-2 shrink-0">
                MCP server configuration (JSON format).
              </Label>
              <textarea
                className="flex-1 min-h-0 w-full rounded-md border p-4 font-mono text-sm bg-background resize-none focus:outline-none focus:ring-2 focus:ring-ring/50"
                value={mcpConfigDraft} onChange={(e) => setMcpConfigDraft(e.target.value)}
                placeholder='{"servers": {}}' />
              {mcpDirty && (
                <div className="shrink-0 mt-3">
                  <Button size="sm" onClick={handleSaveMcp} disabled={saving}>
                    {saving ? t(($) => $.template_editor.saving) : t(($) => $.template_editor.save)}
                  </Button>
                </div>
              )}
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
