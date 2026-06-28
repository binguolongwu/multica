"use client";

import { useState, useMemo } from "react";
import { Plus, Pencil, Trash2, Search, ChevronLeft, ChevronRight } from "lucide-react";
import { useRouter } from "next/navigation";
import {
  useAgentTemplates,
  useDeleteAgentTemplate,
  usePlatformAdmin,
} from "@multica/core/agents/queries";
import type { AgentTemplate } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Badge } from "@multica/ui/components/ui/badge";
import { toast } from "sonner";

const PAGE_SIZE = 20;

export function TemplateLibraryPage() {
  const { data: templates = [], isLoading } = useAgentTemplates();
  const { data: isAdmin } = usePlatformAdmin();
  const deleteMutation = useDeleteAgentTemplate();
  const p = useWorkspacePaths();
  const router = useRouter();

  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);

  const filtered = useMemo(() => {
    if (!search) return templates;
    const q = search.toLowerCase();
    return templates.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.category.toLowerCase().includes(q) ||
        t.tags.some((tag) => tag.toLowerCase().includes(q)),
    );
  }, [templates, search]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const paged = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

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
              onChange={(e) => { setSearch(e.target.value); setPage(0); }}
            />
          </div>
          {isAdmin && (
            <Button onClick={() => router.push(p.templates() + "/new")}>
              <Plus className="h-4 w-4 mr-1" /> New Template
            </Button>
          )}
        </div>
      </div>

      {/* Table */}
      <div className="flex-1 min-h-0 overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/50 text-muted-foreground text-xs sticky top-0">
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
            {isLoading && Array.from({ length: 5 }).map((_, i) => (
              <tr key={i} className="border-b animate-pulse">
                {Array.from({ length: 7 }).map((_, j) => (
                  <td key={j} className="px-3 py-3"><div className="h-4 bg-muted rounded w-3/4" /></td>
                ))}
              </tr>
            ))}
            {!isLoading && paged.map((row) => (
              <tr
                key={row.id}
                className="border-b hover:bg-muted/50 transition-colors cursor-pointer"
                onClick={() => router.push(p.templates() + "/" + row.id)}
              >
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
                <td className="px-5 py-3" onClick={(e) => e.stopPropagation()}>
                  {isAdmin && (
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon-sm" onClick={() => router.push(p.templates() + "/" + row.id)}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost" size="icon-sm"
                        onClick={async () => {
                          if (confirm('Delete template "' + row.name + '"?')) {
                            try { await deleteMutation.mutateAsync(row.id); toast.success("Template deleted"); }
                            catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
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
              <tr><td colSpan={7} className="text-center text-muted-foreground py-12">No templates found.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-3 shrink-0 border-t px-5 py-2 text-xs text-muted-foreground">
          <Button variant="ghost" size="icon-sm" disabled={page === 0} onClick={() => setPage(page - 1)}>
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <span>
            {page + 1} / {totalPages}
          </span>
          <Button variant="ghost" size="icon-sm" disabled={page >= totalPages - 1} onClick={() => setPage(page + 1)}>
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      )}
    </div>
  );
}
