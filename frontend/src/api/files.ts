import { api } from "./client";
import type { FileContentResponse } from "./types";

// Brick 11's endpoint - path must already be repo-relative (use
// repoRelativePath on a raw backend file_path before calling this), since
// the backend rejects absolute paths outright. startLine/endLine are
// optional (1-indexed, inclusive) - when given, only that range comes back
// instead of the whole file, for callers that just want a small preview
// (e.g. the reading-path/concepts snippets on Overview).
export const getFileContent = (repoId: string, path: string, startLine?: number, endLine?: number) => {
  const params = new URLSearchParams({ path });
  if (startLine) params.set("start_line", String(startLine));
  if (endLine) params.set("end_line", String(endLine));
  return api.get<FileContentResponse>(`/files/${repoId}?${params.toString()}`);
};
