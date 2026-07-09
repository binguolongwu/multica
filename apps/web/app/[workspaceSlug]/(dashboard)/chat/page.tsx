"use client";

import React, { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { ChatPage } from "@multica/views/chat/chat-page";

function ChatPageWrapper() {
  const searchParams = useSearchParams();
  const sessionId = searchParams.get("session") ?? undefined;
  return <ChatPage initialSessionId={sessionId} />;
}

export default function ChatRoutePage() {
  return (
    <Suspense fallback={null}>
      <ChatPageWrapper />
    </Suspense>
  );
}
