"use client";

import { useMemo } from "react";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../../common/actor-avatar";
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from "@multica/ui/components/ui/collapsible";
import { useChatStore } from "@multica/core/chat";
import type { ChatSession } from "@multica/core/types/chat";
import { ChevronRight } from "lucide-react";

interface ChatThreadListProps {
  sessions: ChatSession[];
  onDelete: (sessionId: string) => void;
  onStop: (sessionId: string) => void;
  className?: string;
}

interface AgentGroup {
  agentId: string;
  agentName: string;
  agentAvatarUrl: string | null | undefined;
  sessions: ChatSession[];
  totalUnread: number;
}

export function ChatThreadList({ sessions, onDelete, onStop, className }: ChatThreadListProps) {
  const activeSessionId = useChatStore((s) => s.activeSessionId);
  const setActiveSession = useChatStore((s) => s.setActiveSession);

  // Group sessions by agent
  const groups = useMemo<AgentGroup[]>(() => {
    const map = new Map<string, AgentGroup>();
    for (const s of sessions) {
      const existing = map.get(s.agent_id);
      if (existing) {
        existing.sessions.push(s);
        existing.totalUnread += s.unread_count;
      } else {
        map.set(s.agent_id, {
          agentId: s.agent_id,
          agentName: s.agent_name || "Agent",
          agentAvatarUrl: s.agent_avatar_url,
          sessions: [s],
          totalUnread: s.unread_count,
        });
      }
    }
    return Array.from(map.values());
  }, [sessions]);

  // Default open: the group containing the active session
  const defaultOpenIds = useMemo(() => {
    if (!activeSessionId) return new Set<string>();
    const g = groups.find((g) => g.sessions.some((s) => s.id === activeSessionId));
    return g ? new Set([g.agentId]) : new Set<string>();
  }, [activeSessionId, groups]);

  return (
    <div className={cn("flex flex-col h-full", className)}>
      <div className="flex-1 overflow-y-auto">
        {groups.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground text-center">
            No conversations yet
          </div>
        ) : (
          groups.map((group) => (
            <AgentGroupPanel
              key={group.agentId}
              group={group}
              defaultOpen={defaultOpenIds.has(group.agentId)}
              activeSessionId={activeSessionId}
              onSelectSession={(id) => setActiveSession(id)}
              onDelete={onDelete}
              onStop={onStop}
            />
          ))
        )}
      </div>
    </div>
  );
}

function AgentGroupPanel({
  group,
  defaultOpen,
  activeSessionId,
  onSelectSession,
}: {
  group: AgentGroup;
  defaultOpen: boolean;
  activeSessionId: string | null;
  onSelectSession: (id: string) => void;
  onDelete?: (sessionId: string) => void;
  onStop?: (sessionId: string) => void;
}) {
  const hasActiveSession = group.sessions.some((s) => s.id === activeSessionId);

  return (
    <Collapsible defaultOpen={defaultOpen || hasActiveSession}>
      <CollapsibleTrigger
        className={cn(
          "flex items-center gap-2 w-full px-3 py-2 text-sm font-medium",
          "hover:bg-accent/50 transition-colors",
          "group",
        )}
      >
        <ChevronRight className="size-3.5 shrink-0 text-muted-foreground transition-transform group-aria-expanded:rotate-90" />
        <ActorAvatar
          actorType="agent"
          actorId={group.agentId}
          size={18}
          className="shrink-0"
        />
        <span className="flex-1 text-left truncate">{group.agentName}</span>
        <span className="text-xs text-muted-foreground shrink-0">
          {group.sessions.length}
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        {group.sessions.map((session) => (
          <button
            key={session.id}
            type="button"
            onClick={() => onSelectSession(session.id)}
            className={cn(
              "w-full flex items-center justify-between gap-2 pl-11 pr-3 py-2 text-left text-sm transition-colors",
              "hover:bg-accent/50",
              session.id === activeSessionId && "bg-accent",
            )}
          >
            <span
              className={cn(
                "truncate flex-1",
                session.unread_count > 0 ? "font-semibold" : "font-normal",
              )}
            >
              {session.title || "Untitled"}
            </span>
            <span className="text-xs text-muted-foreground shrink-0">
              {session.unread_count > 0 && (
                <span className="inline-flex items-center justify-center size-4 rounded-full bg-destructive text-destructive-foreground text-[10px] font-semibold mr-1">
                  {session.unread_count > 99 ? "99+" : session.unread_count}
                </span>
              )}
              {formatRelativeTime(session.updated_at)}
            </span>
          </button>
        ))}
      </CollapsibleContent>
    </Collapsible>
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
