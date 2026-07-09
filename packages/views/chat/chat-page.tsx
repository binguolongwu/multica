"use client";

import React, { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useChatStore } from "@multica/core/chat";
import { chatSessionsOptions } from "@multica/core/chat/queries";
import {
  useDeleteChatSession,
  useUpdateChatSession,
  useToggleSessionPin,
  useMarkChatSessionRead,
} from "@multica/core/chat/mutations";
import { ChatThreadList } from "./components/chat-thread-list";
import { ChatSessionHeader } from "./components/chat-session-header";
import { ChatEmptyState } from "./components/chat-empty-state";
import { NewChatButton } from "./components/new-chat-button";
import { ArchivedAgentBanner } from "./components/archived-agent-banner";
import { ChatWindow } from "./components/chat-window";

interface ChatPageProps {
  initialSessionId?: string;
}

export function ChatPage({ initialSessionId }: ChatPageProps) {
  const wsId = useWorkspaceId();
  const activeSessionId = useChatStore((s) => s.activeSessionId);
  const setActiveSession = useChatStore((s) => s.setActiveSession);

  const { data: sessions = [] } = useQuery(chatSessionsOptions(wsId));
  const deleteSession = useDeleteChatSession();
  const updateSession = useUpdateChatSession();
  const togglePin = useToggleSessionPin();
  const markRead = useMarkChatSessionRead();

  const activeSession = sessions.find((s) => s.id === activeSessionId) ?? null;

  // Handle initial session from URL param
  useEffect(() => {
    if (initialSessionId && !activeSessionId) {
      setActiveSession(initialSessionId);
    }
  }, [initialSessionId, activeSessionId, setActiveSession]);

  // Mark as read when entering a session
  useEffect(() => {
    if (activeSessionId) {
      markRead.mutate(activeSessionId);
    }
  }, [activeSessionId]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="flex h-full">
      {/* Left pane — Thread List */}
      <div className="w-80 border-r flex flex-col shrink-0">
        <NewChatButton className="m-2" />
        <ChatThreadList
          sessions={sessions}
          onDelete={(id) => {
            deleteSession.mutate(id);
            if (activeSessionId === id) setActiveSession(null);
          }}
          onStop={(_id) => {
            // Stop is handled by the ChatWindow internally
          }}
          className="flex-1"
        />
      </div>

      {/* Right pane — Conversation */}
      <div className="flex-1 flex flex-col min-w-0">
        {activeSession ? (
          <>
            <ChatSessionHeader
              session={activeSession}
              onRename={(title) =>
                updateSession.mutate({ sessionId: activeSession.id, title })
              }
              onPin={() => togglePin.mutate(activeSession.id)}
              onDelete={() => {
                deleteSession.mutate(activeSession.id);
                setActiveSession(null);
              }}
            />
            {activeSession.agent_status === "archived" && (
              <ArchivedAgentBanner agentName={activeSession.agent_name} />
            )}
            <div className="flex-1 relative">
              {/* Reuse ChatWindow for message list + input — it's self-contained and handles all chat logic */}
              <ChatWindow key={activeSession.id} />
            </div>
          </>
        ) : (
          <ChatEmptyState />
        )}
      </div>
    </div>
  );
}
