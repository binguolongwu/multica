import { CAPABILITY_LABELS } from "@multica/core/types";

// CapabilityBadges renders the small pills for a model's capability tags
// (reasoning/tool_use/vision/…). Used on model cards, the fetch/import
// dialog, and the agent model picker so a model's ability boundaries are
// visible at the point of selection. Empty/missing capabilities render nothing.
export function CapabilityBadges({
  capabilities,
  className,
}: {
  capabilities?: string[] | null;
  className?: string;
}) {
  if (!capabilities || capabilities.length === 0) return null;
  return (
    <div className={`flex flex-wrap gap-1 ${className ?? ""}`}>
      {capabilities.map((c) => (
        <span
          key={c}
          className="text-[10px] leading-none px-1 py-0.5 rounded bg-muted text-muted-foreground"
        >
          {CAPABILITY_LABELS[c] ?? c}
        </span>
      ))}
    </div>
  );
}
