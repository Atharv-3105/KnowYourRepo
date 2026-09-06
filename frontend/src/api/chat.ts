import { api } from "./client";
import type { AgentChatRequest, AgentChatResponse } from "./types";

// Only the agentic endpoint is wrapped - plain POST /chat has no tools_used/
// sources and nothing in the frontend plan (Bricks 6-10) uses it.
export const agentChat = (req: AgentChatRequest) =>
  api.post<AgentChatResponse>("/agent/chat", req);
