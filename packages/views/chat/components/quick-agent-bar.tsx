"use client";

import { cn } from "@multica/ui/lib/utils";
import type { PinnedAgent } from "@multica/core/types/chat";

interface QuickAgentBarProps {
  agents: PinnedAgent[];
  onSelect: (agentId: string) => void;
  className?: string;
}

/**
 * Horizontal pinned agent bar shown above thread list.
 * Kept in tree but NOT mounted initially — mount when quick-agent feature is enabled.
 */
export function QuickAgentBar({ agents, onSelect, className }: QuickAgentBarProps) {
  if (agents.length === 0) return null;

  return (
    <div className={cn("flex items-center gap-1 px-2 py-2 overflow-x-auto", className)}>
      {agents.map((pa) => (
        <button
          key={pa.agent_id}
          type="button"
          onClick={() => onSelect(pa.agent_id)}
          className="shrink-0 rounded-full border-2 border-transparent hover:border-primary transition-colors"
          title={pa.agent_id}
        >
          <div className="size-8 rounded-full bg-muted" />
        </button>
      ))}
    </div>
  );
}
