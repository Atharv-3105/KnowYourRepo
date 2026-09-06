import { api } from "./client";
import type { CreateRepoResponse, RepoSummary, SyncRepoResponse } from "./types";

export const listRepos = () => api.get<RepoSummary[]>("/repos");

// Safe to call with an already-ingested repo_url - the backend dedupes by
// URL and reuses the existing repo_id instead of cloning a second copy.
export const createRepo = (repoUrl: string) =>
  api.post<CreateRepoResponse>("/repos", { repo_url: repoUrl });

export const syncRepo = (repoId: string) =>
  api.post<SyncRepoResponse>(`/repos/${repoId}/sync`);
