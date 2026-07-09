"use client";

import React from "react";
import { useChatStore } from "@multica/core/chat";
import { ChatFab } from "./components/chat-fab";
import { ChatWindow } from "./components/chat-window";

/**
 * Mode orchestrator: renders floating chat (FAB + Window) only when
 * chatMode is 'floating'. In 'tab' mode, the ChatPage handles display.
 */
export function FloatingChat() {
  const chatMode = useChatStore((s) => s.chatMode);

  if (chatMode !== "floating") return null;

  return (
    <>
      <ChatFab />
      <ChatWindow />
    </>
  );
}
