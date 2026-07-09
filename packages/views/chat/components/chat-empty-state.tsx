"use client";

import React from "react";
import { Bot, Lightbulb, ListChecks, Search } from "lucide-react";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";

interface ChatEmptyStateProps {
  agentName?: string;
  agentAvatarUrl?: string | null;
  agentDescription?: string | null;
  onStarterClick?: (prompt: string) => void;
}

const DEFAULT_STARTERS = [
  { icon: Lightbulb, label: "What can you do?" },
  { icon: ListChecks, label: "Help me plan a task" },
  { icon: Search, label: "Find information for me" },
];

export function ChatEmptyState({
  agentName,
  agentAvatarUrl,
  agentDescription,
  onStarterClick,
}: ChatEmptyStateProps) {
  const starters = DEFAULT_STARTERS;

  return (
    <div className="flex flex-col items-center justify-center h-full px-6 py-12">
      <ActorAvatar
        actor={{ type: "agent", id: "", name: agentName, avatar_url: agentAvatarUrl }}
        size="lg"
        className="mb-4"
      />
      <h2 className="text-lg font-semibold mb-1">
        {agentName ? `Chat with ${agentName}` : "Select an agent to start chatting"}
      </h2>
      {agentDescription && (
        <p className="text-sm text-muted-foreground text-center max-w-sm mb-6">
          {agentDescription}
        </p>
      )}
      {agentName && (
        <div className="flex flex-col gap-2 w-full max-w-xs">
          {starters.map((starter) => (
            <button
              key={starter.label}
              type="button"
              onClick={() => onStarterClick?.(starter.label)}
              className="flex items-center gap-2 px-4 py-2 text-sm rounded-lg border hover:bg-accent transition-colors"
            >
              <starter.icon className="size-4 text-muted-foreground" />
              {starter.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
