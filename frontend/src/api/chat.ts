import { api } from "./client";
import type { AgentChatRequest, AgentChatResponse } from "./types";

// POST /chat now runs the single answer.Service retrieval path (see
// docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md) -
// the old separate /agent/chat endpoint no longer exists.
export const agentChat = (req: AgentChatRequest) =>
  api.post<AgentChatResponse>("/chat", req);
