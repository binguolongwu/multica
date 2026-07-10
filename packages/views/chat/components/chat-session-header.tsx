"use client";

import { useState, useCallback } from "react";
import { MoreHorizontal, Pin, PinOff, Trash2 } from "lucide-react";
import { ActorAvatar } from "../../common/actor-avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import type { ChatSession } from "@multica/core/types/chat";

interface ChatSessionHeaderProps {
  session: ChatSession;
  onRename: (title: string) => void;
  onPin: () => void;
  onDelete: () => void;
}

export function ChatSessionHeader({ session, onRename, onPin, onDelete }: ChatSessionHeaderProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(session.title);

  const handleSubmit = useCallback(() => {
    if (draft.trim() && draft !== session.title) {
      onRename(draft.trim());
    }
    setEditing(false);
  }, [draft, session.title, onRename]);

  return (
    <div className="flex items-center gap-3 px-4 py-3 border-b shrink-0">
      <ActorAvatar
        actorType="agent"
        actorId={session.agent_id}
        size={20}
      />
      <div className="flex-1 min-w-0">
        {editing ? (
          <input
            type="text"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleSubmit();
              if (e.key === "Escape") {
                setDraft(session.title);
                setEditing(false);
              }
            }}
            onBlur={handleSubmit}
            className="w-full text-sm font-medium bg-transparent border-b border-primary outline-none"
            // eslint-disable-next-line jsx-a11y/no-autofocus
            autoFocus
          />
        ) : (
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="text-sm font-medium truncate hover:text-primary cursor-text max-w-full"
          >
            {session.title || `Chat with ${session.agent_name || "Agent"}`}
          </button>
        )}
        {session.agent_status === "archived" && (
          <span className="text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded ml-1 align-middle">
            Archived
          </span>
        )}
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <button type="button" className="p-1 rounded hover:bg-muted shrink-0">
              <MoreHorizontal className="size-4" />
            </button>
          }
        />
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => setEditing(true)}>Rename</DropdownMenuItem>
          <DropdownMenuItem onClick={onPin}>
            {session.is_pinned ? (
              <>
                <PinOff className="size-4 mr-2" /> Unpin
              </>
            ) : (
              <>
                <Pin className="size-4 mr-2" /> Pin to top
              </>
            )}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={onDelete} className="text-destructive">
            <Trash2 className="size-4 mr-2" /> Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
