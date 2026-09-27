import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import type { ThemedToken } from "@shikijs/types";
import { ApiError, getFileContent } from "../api";
import { getHighlighter } from "../lib/highlighter";
import { languageFromPath } from "../lib/language";
import { useCodeView } from "../lib/useCodeView";
import { Dialog } from "./ui/Dialog";
import CopyableLocation from "./CopyableLocation";
import ErrorState from "./ErrorState";
import { SkeletonLine } from "./Skeleton";

interface CodeViewPanelProps {
  repoId: string;
}

// Slide-in panel (Brick 14's Dialog, panel variant) rendering real,
// syntax-highlighted source fetched from Brick 11's file-content endpoint.
// Uses the curated highlighter's codeToTokensBase (structured per-line
// tokens, synchronous once the highlighter is loaded), not codeToHtml,
// because this panel needs its own line-number gutter and a highlighted
// line range - splitting Shiki's raw HTML output by line would be fragile
// against tags spanning line boundaries.
export default function CodeViewPanel({ repoId }: CodeViewPanelProps) {
  const { target, close } = useCodeView();
  const highlightRef = useRef<HTMLTableRowElement>(null);

  const fileQuery = useQuery({
    queryKey: ["file", repoId, target?.path],
    queryFn: () => getFileContent(repoId, target!.path),
    enabled: Boolean(target),
  });

  // Tokenizing is derived from the file content, so it's a query keyed on
  // that content rather than an effect copying results into state.
  const file = fileQuery.data;
  const highlightQuery = useQuery({
    queryKey: ["highlight", repoId, file?.path, fileQuery.dataUpdatedAt],
    queryFn: async (): Promise<ThemedToken[][]> => {
      const highlighter = await getHighlighter();
      return highlighter.codeToTokensBase(file!.content, {
        lang: languageFromPath(file!.path),
        theme: "github-light",
      });
    },
    enabled: Boolean(file),
    staleTime: Infinity,
  });
  const lines = file ? (highlightQuery.data ?? null) : null;

  useEffect(() => {
    if (lines && target?.startLine) {
      highlightRef.current?.scrollIntoView({ block: "center" });
    }
  }, [lines, target?.startLine]);

  return (
    <Dialog open={Boolean(target)} onOpenChange={(open) => !open && close()} variant="panel" srTitle="File viewer">
      {target && (
        <div className="flex h-full flex-col">
          <div className="flex items-center justify-between gap-2 border-b border-line-faint px-4 py-3">
            <span className="truncate font-mono text-sm text-ink">{target.path}</span>
            <CopyableLocation filePath={target.path} startLine={target.startLine} endLine={target.endLine} />
          </div>

          <div className="flex-1 overflow-auto">
            {fileQuery.isLoading && (
              <div className="space-y-2 p-4">
                {[0, 1, 2, 3, 4, 5].map((i) => (
                  <SkeletonLine key={i} className={i % 2 === 0 ? "w-full" : "w-3/4"} />
                ))}
              </div>
            )}

            {fileQuery.isError && (
              <div className="p-4">
                <ErrorState
                  message={fileQuery.error instanceof ApiError ? fileQuery.error.message : "Failed to load file."}
                  onRetry={() => fileQuery.refetch()}
                />
              </div>
            )}

            {lines && (
              <table className="w-full border-collapse font-mono text-xs">
                <tbody>
                  {lines.map((lineTokens, i) => {
                    const lineNo = i + 1;
                    const isHighlighted =
                      target.startLine &&
                      lineNo >= target.startLine &&
                      lineNo <= (target.endLine ?? target.startLine);

                    return (
                      <tr
                        key={i}
                        ref={isHighlighted && lineNo === target.startLine ? highlightRef : undefined}
                        className={isHighlighted ? "bg-accent/10" : undefined}
                      >
                        <td className="select-none border-r border-line-faint px-3 py-0.5 text-right text-ink-faint">
                          {lineNo}
                        </td>
                        <td className="whitespace-pre px-3 py-0.5">
                          {lineTokens.map((token, ti) => (
                            <span key={ti} style={{ color: token.color }}>
                              {token.content}
                            </span>
                          ))}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}
    </Dialog>
  );
}
