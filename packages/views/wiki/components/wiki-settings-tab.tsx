"use client";

import { BookOpen } from "lucide-react";

export function WikiSettingsTab() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <BookOpen className="h-5 w-5" />
        <div>
          <h3 className="text-sm font-semibold">Wiki Knowledge Base</h3>
          <p className="text-xs text-muted-foreground">
            LLM-maintained workspace wiki for knowledge sharing and agent learning.
          </p>
        </div>
      </div>
      <p className="text-sm text-muted-foreground">
        Manage wiki spaces, configure auto-capture settings, and view operations from the Wiki page.
      </p>
    </div>
  );
}
