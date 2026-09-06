import { api } from "./client";
import type { BoundedGraphResponse } from "./types";

// Wraps the bounded, repo-scoped call graph (Brick 3), not the legacy
// unscoped GET /graph - that one mixes edges across every ingested repo
// and has no place in a per-repo UI.
export const getBoundedGraph = (repoId: string, symbol: string, depth = 2) =>
  api.get<BoundedGraphResponse>(
    `/graph/${repoId}?symbol=${encodeURIComponent(symbol)}&depth=${depth}`,
  );
