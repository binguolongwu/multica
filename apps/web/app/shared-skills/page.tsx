"use client";

import { useMemo } from "react";
import { BookOpen, Plus } from "lucide-react";
import type { SkillSummary } from "@multica/core/types";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { usePlatformAdmin } from "@multica/core/agents/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useNavigation } from "@multica/views/navigation";
import { PageHeader } from "@multica/views/layout/page-header";
import { useT, useTimeAgo } from "@multica/views/i18n";

export default function SharedSkillsPage() {
  const { t } = useT("skills");
  const timeAgo = useTimeAgo();
  const navigation = useNavigation();
  const { data: isAdmin } = usePlatformAdmin();

  const { data: skills = [], isLoading } = useQuery({
    queryKey: ["platform-skills"],
    queryFn: () => api.listPlatformSkills(),
  });

  return (
    <div className="flex flex-1 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex items-center gap-2">
          <BookOpen className="h-4 w-4 text-muted-foreground" />
          <h1 className="text-sm font-medium">Shared Skills</h1>
          <span className="font-mono text-xs tabular-nums text-muted-foreground/70">
            {skills.length}
          </span>
        </div>
        {isAdmin && (
          <Button
            type="button" size="sm" variant="outline" className="h-8 gap-1"
          >
            <Plus className="h-3.5 w-3.5" />
            New Skill
          </Button>
        )}
      </PageHeader>

      {isLoading ? (
        <div className="space-y-2 p-6">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : (
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
              {skills.map((skill: SkillSummary) => (
                <tr
                  key={skill.id}
                  className="border-b hover:bg-accent/50 cursor-pointer"
                  onClick={() => navigation.push(`/shared-skills/${skill.id}`)}
                >
                  <td className="px-5 py-3">
                    <span className="text-sm font-medium truncate">{skill.name}</span>
                  </td>
                  <td className="px-3 py-3 hidden lg:table-cell">
                    <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium ${
                      skill.skill_type === 'builtin'
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
      )}
    </div>
  );
}
