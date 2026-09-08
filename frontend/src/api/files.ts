import { api } from "./client";
import type { FileContentResponse } from "./types";

// Brick 11's endpoint - path must already be repo-relative (use
// repoRelativePath on a raw backend file_path before calling this), since
// the backend rejects absolute paths outright.
export const getFileContent = (repoId: string, path: string) =>
  api.get<FileContentResponse>(`/files/${repoId}?path=${encodeURIComponent(path)}`);
