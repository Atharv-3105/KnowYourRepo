import { api } from "./client";
import type { SearchResponse } from "./types";

// Real semantic search, previously wired into nothing in the frontend -
// the Command Palette (Brick 17) is its first consumer.
export const search = (repoId: string, query: string) =>
  api.post<SearchResponse>("/search", { repo_id: repoId, query });
