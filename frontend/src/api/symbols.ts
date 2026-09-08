import { api } from "./client";
import type { SymbolEntry } from "./types";

// The repo's real, complete symbol index (Brick 13) - not the heuristic
// subset GET /architecture/:repoID's `components` returns.
export const listSymbols = (repoId: string, search = "") =>
  api.get<SymbolEntry[]>(`/symbols/${repoId}?search=${encodeURIComponent(search)}`);
