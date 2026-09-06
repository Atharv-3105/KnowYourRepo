import { api } from "./client";
import type { ArchitectureSummary } from "./types";

export const getArchitecture = (repoId: string) =>
  api.get<ArchitectureSummary>(`/architecture/${repoId}`);
