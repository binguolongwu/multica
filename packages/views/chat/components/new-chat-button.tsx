"use client";

import React, { useState, useCallback } from "react";
import { Plus, ChevronDown } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { Popover, PopoverContent, PopoverTrigger } from "@multica/ui/components/ui/popover";
import { useWorkspaceId } from "@multica/core/hooks";
import { useChatStore } from "@multica/core/chat";
import { useCreateChatSession } from "@multica/core/chat/mutations";
import { agentListOptions } from "@multica/core/workspace/queries";
import type { Agent } from "@multica/core/types";

interface NewChatButtonProps {
  className?: string;
}

export function NewChatButton({ className }: NewChatButtonProps) {
  const wsId = useWorkspaceId();
  const [open, setOpen] = useState(false);
  const setActiveSession = useChatStore((s) => s.setActiveSession);
  const setSelectedAgentId = useChatStore((s) => s.setSelectedAgentId);
  const createSession = useCreateChatSession();
  const { data: agents = [] } = useQuery(agentListOptions(wsId));

  const handleSelectAgent = useCallback(
    async (agent: Agent) => {
      setOpen(false);
      setSelectedAgentId(agent.id);
      const result = await createSession.mutateAsync({ agent_id: agent.id });
      if (result?.id) {
        setActiveSession(result.id);
      }
    },
    [createSession, setActiveSession, setSelectedAgentId],
  );

  const chatAgents = agents.filter((a: Agent) => a.status !== "deleted");

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className={cn(
            "flex items-center gap-2 w-full px-3 py-2 text-sm font-medium rounded-lg",
            "hover:bg-accent transition-colors",
            className,
          )}
        >
          <Plus className="size-4" />
          <span className="flex-1 text-left">New Chat</span>
          <ChevronDown className="size-3 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-64 p-1" align="start">
        <div className="max-h-72 overflow-y-auto">
          {chatAgents.length === 0 ? (
            <div className="px-3 py-2 text-sm text-muted-foreground">No agents available</div>
          ) : (
            chatAgents.map((agent: Agent) => (
              <button
                key={agent.id}
                type="button"
                onClick={() => handleSelectAgent(agent)}
                className="flex items-center gap-3 w-full px-3 py-2 text-sm rounded hover:bg-accent transition-colors"
              >
                <ActorAvatar
                  actor={{
                    type: "agent",
                    id: agent.id,
                    name: agent.name,
                    avatar_url: agent.avatar_url,
                  }}
                  size="sm"
                />
                <span className="truncate">{agent.name}</span>
              </button>
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
