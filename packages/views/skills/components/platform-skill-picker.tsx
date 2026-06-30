"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { Skill } from "@multica/core/types";
import { skillListOptions } from "@multica/core/workspace/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { Loader2, Check, Globe } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
import { toast } from "sonner";

export function PlatformSkillPicker({
  onInstalled,
  onCancel,
}: {
  onInstalled: (skill: Skill) => void;
  onCancel: () => void;
}) {
  const wsId = useWorkspaceId();
  const { data: platformSkills, isLoading } = useQuery({
    queryKey: ["platform-skills"],
    queryFn: () => api.listPlatformSkills(),
  });
  const { data: workspaceSkills } = useQuery(skillListOptions(wsId));
  const [installing, setInstalling] = useState<string | null>(null);

  const workspaceSkillNames = new Set(
    (workspaceSkills ?? []).map((s) => s.name),
  );

  const handleInstall = async (skillId: string) => {
    setInstalling(skillId);
    try {
      const skill = await api.installSkill(skillId);
      toast.success(`Installed "${skill.name}"`);
      onInstalled(skill);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to install");
      setInstalling(null);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="h-5 w-5 animate-spin" />
      </div>
    );
  }

  return (
    <div className="flex flex-col max-h-[400px]">
      <div className="flex-1 overflow-y-auto p-4 space-y-1">
        {(platformSkills ?? []).map((skill) => {
          const installed = workspaceSkillNames.has(skill.name);
          return (
            <div
              key={skill.id}
              className={cn(
                "flex items-center justify-between gap-2 rounded-md border px-3 py-2.5",
                installed && "opacity-60",
              )}
            >
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="text-sm font-medium truncate">
                    {skill.name}
                  </span>
                  {skill.is_builtin && (
                    <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                      Built-in
                    </span>
                  )}
                </div>
                <div className="text-xs text-muted-foreground line-clamp-1 mt-0.5">
                  {skill.description}
                </div>
              </div>
              {skill.is_builtin ? (
                <span className="text-xs text-muted-foreground shrink-0">
                  Auto
                </span>
              ) : installed ? (
                <Check className="h-4 w-4 text-green-500 shrink-0" />
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  disabled={installing === skill.id}
                  onClick={() => handleInstall(skill.id)}
                >
                  {installing === skill.id ? (
                    <Loader2 className="h-3 w-3 animate-spin" />
                  ) : (
                    "Install"
                  )}
                </Button>
              )}
            </div>
          );
        })}
      </div>
      <div className="flex shrink-0 items-center justify-end gap-2 border-t bg-muted/30 px-5 py-3">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  );
}
