// Maps the real tool names a chat answer can report in tools_used
// ("semantic" always runs; "architecture" only when the question's
// phrasing calls for an overview - see answer.Service.Answer and
// api/internal/api/agent_chat.go's toolsUsed) to the Field Notes
// per-tool accent tokens, so the badge encodes real information about
// which capability contributed to this answer, not decoration.
// "semantic" reuses --color-accent since it's the retrieval path that
// always runs, not a fourth invented hue.
const TOOL_BADGE_CLASSES: Record<string, string> = {
  semantic: "border-accent/40 text-accent",
  architecture: "border-tool-architecture/40 text-tool-architecture",
};

export function toolBadgeClasses(tool: string): string {
  return TOOL_BADGE_CLASSES[tool] ?? "border-line-faint text-ink-dim";
}
