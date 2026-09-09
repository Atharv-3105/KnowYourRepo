// Maps the real tool names a chat answer can report in tools_used
// ("semantic" and "lexical" run unconditionally; "architecture" only when
// the question's phrasing calls for an overview - see answer.Service.Answer
// and api/internal/api/agent_chat.go's toolsUsed) to the Field Notes
// per-tool accent tokens, so the badge encodes real information about
// which capability contributed to this answer, not decoration.
// "semantic" reuses --color-accent since it's the retrieval path that
// always runs, not a fourth invented hue. "lexical" reuses
// --color-tool-graph - the lexical fallback IS a direct symbol-table/
// call-graph lookup (retrieval.HybridRetriever.LexicalSearch), so the
// token's original meaning fits it even better than the deleted GraphTool
// it was first defined for.
const TOOL_BADGE_CLASSES: Record<string, string> = {
  semantic: "border-accent/40 text-accent",
  lexical: "border-tool-graph/40 text-tool-graph",
  architecture: "border-tool-architecture/40 text-tool-architecture",
};

export function toolBadgeClasses(tool: string): string {
  return TOOL_BADGE_CLASSES[tool] ?? "border-line-faint text-ink-dim";
}
