// Maps the agent's real tool names (api/internal/agent/models.go's
// ToolName - "semantic" | "graph" | "architecture" | "memory") to the
// Field Notes per-tool accent tokens, so a chat answer's "tools used"
// badges encode which capability actually answered - real information,
// not decoration. "semantic" reuses --color-accent rather than a fourth
// hue: it's the planner's own default/fallback tool (see hybrid_planner.go),
// so it doubling as "the" accent color is grounded in the backend's logic.
// "memory" (conversation history, not a retrieval capability) and any
// unrecognized name fall back to the neutral badge styling used everywhere
// else in the app.
const TOOL_BADGE_CLASSES: Record<string, string> = {
  semantic: "border-accent/40 text-accent",
  graph: "border-tool-graph/40 text-tool-graph",
  architecture: "border-tool-architecture/40 text-tool-architecture",
};

export function toolBadgeClasses(tool: string): string {
  return TOOL_BADGE_CLASSES[tool] ?? "border-line-faint text-ink-dim";
}
