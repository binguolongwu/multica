"use client";

import React from "react";
import { AlertTriangle } from "lucide-react";

interface ArchivedAgentBannerProps {
  agentName?: string;
}

export function ArchivedAgentBanner({ agentName }: ArchivedAgentBannerProps) {
  return (
    <div className="flex items-center gap-3 px-4 py-2 bg-muted/50 border-b text-sm">
      <AlertTriangle className="size-4 text-amber-500 shrink-0" />
      <span className="flex-1 text-muted-foreground">
        Agent &ldquo;{agentName || "Unknown"}&rdquo; has been archived. You can still read this conversation.
      </span>
    </div>
  );
}
