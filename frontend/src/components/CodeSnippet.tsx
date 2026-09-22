import { useQuery } from "@tanstack/react-query";
import { getFileContent } from "../api";
import { repoRelativePath } from "../lib/path";
import { SkeletonBlock } from "./Skeleton";

// A genuine preview, not the whole function body - reading the rest is
// already one click away via the existing "view in graph"/"ask about this"
// links. Keeps an inline excerpt from dominating the page for a large
// function.
const MAX_PREVIEW_LINES = 15;

interface CodeSnippetProps {
  repoId: string;
  // Raw backend file_path (absolute clone-directory path) - converted to
  // repo-relative internally, matching every other file_path consumer on
  // this page.
  filePath: string;
  startLine: number;
  endLine: number;
}

// Renders a small, read-only code excerpt for a reading-path step or a
// resolved concept citation. Silently renders nothing on any failure (no
// file path, no line range, fetch error) - this is a nice-to-have preview
// alongside a real citation/link, not something worth showing an error
// state for.
export default function CodeSnippet({ repoId, filePath, startLine, endLine }: CodeSnippetProps) {
  const previewEnd = Math.min(endLine, startLine + MAX_PREVIEW_LINES - 1);
  const relPath = filePath ? repoRelativePath(filePath, repoId) : "";

  const { data, isLoading } = useQuery({
    queryKey: ["file-snippet", repoId, relPath, startLine, previewEnd],
    queryFn: () => getFileContent(repoId, relPath, startLine, previewEnd),
    enabled: Boolean(repoId && relPath && startLine > 0 && endLine > 0),
    // Source at a given line range doesn't change without a re-ingestion,
    // which would invalidate the whole page's data anyway - safe to treat
    // as immutable for the life of this query cache.
    staleTime: Infinity,
    retry: false,
  });

  if (!relPath || startLine <= 0 || endLine <= 0) return null;
  if (isLoading) return <SkeletonBlock className="mt-1.5 h-16 w-full" />;
  if (!data || !data.content) return null;

  const truncated = endLine > previewEnd;

  return (
    <pre className="mt-1.5 overflow-x-auto border border-line-faint bg-page-deep px-2.5 py-2 font-mono text-xs text-ink-dim">
      <code>
        {data.content}
        {truncated && "\n…"}
      </code>
    </pre>
  );
}
