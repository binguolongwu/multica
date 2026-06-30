"use client";

import { useMemo, useState } from "react";
import { BookOpen, Plus, ChevronLeft, ChevronRight, Search, Filter, ArrowUpDown } from "lucide-react";
import type { SkillSummary } from "@multica/core/types";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { usePlatformAdmin } from "@multica/core/agents/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useNavigation } from "@multica/views/navigation";
import { useT, useTimeAgo } from "@multica/views/i18n";

const PAGE_SIZE = 20;

type SortField = "name" | "updated";

export default function SharedSkillsPage() {
  const { t } = useT("skills");
  const timeAgo = useTimeAgo();
  const navigation = useNavigation();
  const { data: isAdmin } = usePlatformAdmin();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState<Set<"builtin" | "platform">>(new Set(["builtin", "platform"]));
  const [sortField, setSortField] = useState<SortField>("updated");

  const { data: skills = [], isLoading } = useQuery({
    queryKey: ["platform-skills"],
    queryFn: () => api.listPlatformSkills(),
  });

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
	let result = skills;
    if (typeFilter.size < 2) {
      if (typeFilter.has("builtin")) result = result.filter((s) => s.is_builtin);
      else if (typeFilter.has("platform")) result = result.filter((s) => !s.is_builtin);
    }
    if (q) result = result.filter((s) => s.name.toLowerCase().includes(q));
    result.sort((a, b) => {
      if (sortField === "name") return a.name.localeCompare(b.name);
      return Date.parse(b.updated_at) - Date.parse(a.updated_at);
    });
    return result;
  }, [skills, search, typeFilter, sortField]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const pagedSkills = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE;
    return filtered.slice(start, start + PAGE_SIZE);
  }, [filtered, page]);

  if (page > totalPages) setPage(1);

  const toggleTypeFilter = (type: "builtin" | "platform") => {
    setTypeFilter((prev) => {
      const next = new Set(prev);
      if (next.has(type)) {
        if (next.size > 1) next.delete(type);
      } else {
        next.add(type);
      }
      return next;
    });
    setPage(1);
  };

  return (
    <div className="flex flex-1 flex-col">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-5">
        <div className="flex items-center gap-2">
          <BookOpen className="h-4 w-4 text-muted-foreground" />
          <h1 className="text-sm font-medium">Shared Skills</h1>
          <span className="font-mono text-xs tabular-nums text-muted-foreground/70">
            {filtered.length}
          </span>
        </div>
        {isAdmin && (
          <Button type="button" size="sm" variant="outline" className="h-8 gap-1">
            <Plus className="h-3.5 w-3.5" />
            New Skill
          </Button>
        )}
      </div>

      {/* Toolbar */}
      <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b px-5">
        <div className="relative hidden md:block">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-8 w-64 pl-8 text-sm"
            placeholder="搜索 skill..."
            value={search}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => { setSearch(e.target.value); setPage(1); }}
          />
        </div>
        <div className="flex items-center gap-1">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="outline" size="sm" className="h-8 gap-1 px-2 text-muted-foreground md:px-2.5">
                  <Filter className="size-3.5" />
                  <span className="hidden md:inline">筛选</span>
                </Button>
              }
            />
            <DropdownMenuContent className="w-40">
              <DropdownMenuCheckboxItem
                checked={typeFilter.has("builtin")}
                onCheckedChange={() => toggleTypeFilter("builtin")}
              >
                <span className="rounded-md bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">builtin</span>
                <span className="ml-auto text-xs text-muted-foreground">{skills.filter((s) => s.is_builtin).length}</span>
              </DropdownMenuCheckboxItem>
              <DropdownMenuCheckboxItem
                checked={typeFilter.has("platform")}
                onCheckedChange={() => toggleTypeFilter("platform")}
              >
                <span className="rounded-md bg-purple-100 px-1.5 py-0.5 text-[10px] font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">platform</span>
                <span className="ml-auto text-xs text-muted-foreground">{skills.filter((s) => !s.is_builtin).length}</span>
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="outline" size="sm" className="h-8 gap-1 px-2 text-muted-foreground md:px-2.5">
                  <ArrowUpDown className="size-3.5" />
                  <span className="hidden md:inline">{sortField === "name" ? "名称" : "更新时间"}</span>
                </Button>
              }
            />
            <DropdownMenuContent className="w-36">
              <DropdownMenuCheckboxItem
                checked={sortField === "updated"}
                onCheckedChange={() => setSortField("updated")}
              >
                更新时间
              </DropdownMenuCheckboxItem>
              <DropdownMenuCheckboxItem
                checked={sortField === "name"}
                onCheckedChange={() => setSortField("name")}
              >
                名称
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Body */}
      {isLoading ? (
        <div className="space-y-2 p-6">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="min-h-0 flex-1 overflow-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b text-left text-xs text-muted-foreground">
                  <th className="px-5 py-2 font-medium">Name</th>
                  <th className="px-3 py-2 font-medium hidden lg:table-cell">Type</th>
                  <th className="px-3 py-2 font-medium hidden lg:table-cell">Description</th>
                  <th className="px-3 py-2 font-medium hidden md:table-cell">Updated</th>
                </tr>
              </thead>
              <tbody>
                {pagedSkills.map((skill: SkillSummary) => (
                  <tr
                    key={skill.id}
                    className="border-b hover:bg-accent/50 cursor-pointer"
                    onClick={() => navigation.push(`shared-skills/${skill.id}`)}
                  >
                    <td className="px-5 py-3">
                      <span className="text-sm font-medium truncate">{skill.name}</span>
                    </td>
                    <td className="px-3 py-3 hidden lg:table-cell">
                      <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium ${
                        skill.is_builtin
                          ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                          : "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                      }`}>
                        {skill.skill_type}
                      </span>
                    </td>
                    <td className="px-3 py-3 hidden lg:table-cell">
                      <span className="text-xs text-muted-foreground truncate max-w-[300px] block">
                        {skill.description}
                      </span>
                    </td>
                    <td className="px-3 py-3 hidden md:table-cell">
                      <span className="text-xs text-muted-foreground">
                        {timeAgo(skill.updated_at)}
                      </span>
                    </td>
                  </tr>
                ))}
                {filtered.length === 0 && skills.length > 0 && (
                  <tr>
                    <td colSpan={4} className="px-5 py-16 text-center text-sm text-muted-foreground">
                      No skills match your filters.
                    </td>
                  </tr>
                )}
                {skills.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-5 py-16 text-center text-sm text-muted-foreground">
                      No shared skills yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 border-t py-3 shrink-0">
              <Button
                variant="outline" size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </Button>
              <span className="text-xs text-muted-foreground">
                {page} / {totalPages}
              </span>
              <Button
                variant="outline" size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
