"use client";

import React from "react";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { useChatStore } from "@multica/core/chat";
import type { ChatSession } from "@multica/core/types/chat";
import { X, Square } from "lucide-react";

interface ChatThreadListProps {
  sessions: ChatSession[];
  onDelete: (sessionId: string) => void;
  onStop: (sessionId: string) => void;
  className?: string;
}

export function ChatThreadList({ sessions, onDelete, onStop, className }: ChatThreadListProps) {
  const activeSessionId = useChatStore((s) => s.activeSessionId);
  const setActiveSession = useChatStore((s) => s.setActiveSession);

  return (
    <div className={cn("flex flex-col h-full", className)}>
      <div className="flex-1 overflow-y-auto">
        {sessions.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground text-center">
            No conversations yet
          </div>
        ) : (
          sessions.map((session) => (
            <ChatThreadRow
              key={session.id}
              session={session}
              isActive={session.id === activeSessionId}
              onClick={() => setActiveSession(session.id)}
              onDelete={() => onDelete(session.id)}
              onStop={() => onStop(session.id)}
            />
          ))
        )}
      </div>
    </div>
  );
}

function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

function ChatThreadRow({
  session,
  isActive,
  onClick,
  onDelete,
  onStop,
}: {
  session: ChatSession;
  isActive: boolean;
  onClick: () => void;
  onDelete: () => void;
  onStop: () => void;
}) {
  const [hovered, setHovered] = React.useState(false);
  const time = formatRelativeTime(session.updated_at);

  return (
    <button
      type="button"
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      className={cn(
        "w-full flex items-start gap-3 px-3 py-3 text-left transition-colors",
        "hover:bg-accent/50",
        isActive && "bg-accent",
      )}
    >
      <ActorAvatar
        actor={{
          type: "agent",
          id: session.agent_id,
          name: session.agent_name,
          avatar_url: session.agent_avatar_url ?? null,
        }}
        size="sm"
        className="shrink-0 mt-0.5"
      />
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span
            className={cn(
              "text-sm truncate",
              session.unread_count > 0 ? "font-semibold" : "font-normal",
            )}
          >
            {session.title || `Chat with ${session.agent_name || "Agent"}`}
          </span>
          {hovered ? (
            <div className="flex items-center gap-1 shrink-0">
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    role="button"
                    tabIndex={0}
                    onClick={(e) => {
                      e.stopPropagation();
                      onStop();
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.stopPropagation();
                        onStop();
                      }
                    }}
                    className="p-0.5 rounded hover:bg-muted cursor-pointer"
                  >
                    <Square className="size-3.5" />
                  </span>
                </TooltipTrigger>
                <TooltipContent>Stop</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    role="button"
                    tabIndex={0}
                    onClick={(e) => {
                      e.stopPropagation();
                      onDelete();
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.stopPropagation();
                        onDelete();
                      }
                    }}
                    className="p-0.5 rounded hover:bg-destructive/20 cursor-pointer"
                  >
                    <X className="size-3.5" />
                  </span>
                </TooltipTrigger>
                <TooltipContent>Delete</TooltipContent>
              </Tooltip>
            </div>
          ) : (
            <span className="text-xs text-muted-foreground shrink-0">
              {session.unread_count > 0 && (
                <span className="inline-flex items-center justify-center size-5 rounded-full bg-destructive text-destructive-foreground text-[10px] font-semibold mr-1">
                  {session.unread_count > 99 ? "99+" : session.unread_count}
                </span>
              )}
              {time}
            </span>
          )}
        </div>
      </div>
    </button>
  );
}
