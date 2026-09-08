import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useParams } from "react-router-dom";
import { ApiError, getArchitecture } from "../api";
import type { ArchitectureComponent } from "../api";
import CopyableLocation from "../components/CopyableLocation";
import ErrorState from "../components/ErrorState";
import OpenInCodeView from "../components/OpenInCodeView";
import { SkeletonBlock, SkeletonLine } from "../components/Skeleton";
import StatReading from "../components/StatReading";
import { topLevelDir } from "../lib/path";

// "Detected" is the only real label used here (not "Inferred"/"Unknown")
// - the backend has no confidence scoring, so everything shown either was
// found by the extraction pipeline or isn't shown at all. Components are
// grouped by top-level directory (reusing Overview's topLevelDir utility)
// so a repo with components spread across many files reads as structure,
// not a flat, undifferentiated list.
export default function Architecture() {
  const { repoId } = useParams<{ repoId: string }>();

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ["architecture", repoId],
    queryFn: () => getArchitecture(repoId!),
    enabled: Boolean(repoId),
  });

  const componentsByDir = useMemo(() => {
    if (!data || !repoId) return [];

    const groups = new Map<string, ArchitectureComponent[]>();
    for (const c of data.components) {
      const dir = topLevelDir(c.file_path, repoId);
      const list = groups.get(dir) ?? [];
      list.push(c);
      groups.set(dir, list);
    }

    return Array.from(groups.entries()).sort(([a], [b]) => {
      if (a === "(root)") return -1;
      if (b === "(root)") return 1;
      return a.localeCompare(b);
    });
  }, [data, repoId]);

  if (isLoading) {
    return (
      <div className="space-y-10">
        <SkeletonBlock className="h-16 w-full" />
        <div className="space-y-2">
          <SkeletonLine className="w-24" />
          <SkeletonLine className="w-full" />
          <SkeletonLine className="w-full" />
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <ErrorState
        message={error instanceof ApiError ? error.message : "Failed to load architecture."}
        onRetry={() => refetch()}
      />
    );
  }

  if (!data || !repoId) return null;

  return (
    <div className="space-y-10">
      <div className="flex divide-x divide-line-faint border-y border-line-faint">
        <StatReading label="Files" value={data.statistics.file_count} />
        <StatReading label="Symbols" value={data.statistics.symbol_count} />
        <StatReading label="Call edges" value={data.statistics.call_edges} />
      </div>

      {data.languages.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {data.languages.map((lang) => (
            <span key={lang} className="border border-line-faint px-2 py-0.5 font-mono text-xs text-ink-dim">
              {lang}
            </span>
          ))}
        </div>
      )}

      <section>
        <h2 className="text-sm text-ink-dim">Detected entrypoints</h2>
        {data.entrypoints.length === 0 ? (
          <p className="mt-2 text-sm text-ink-faint">None detected.</p>
        ) : (
          <ul className="mt-2 divide-y divide-line-faint border-t border-line-faint">
            {data.entrypoints.map((ep, i) => (
              <li
                key={`${ep.file_path}-${ep.name}-${i}`}
                className="flex flex-wrap items-center gap-2 py-2"
              >
                <span className="font-mono text-sm text-ink">{ep.name}</span>
                <span className="text-xs text-ink-dim">{ep.type}</span>
                <CopyableLocation filePath={ep.file_path} />
                <OpenInCodeView repoId={repoId} filePath={ep.file_path} />
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <h2 className="text-sm text-ink-dim">Detected components ({data.components.length})</h2>
        {componentsByDir.length === 0 ? (
          <p className="mt-2 text-sm text-ink-faint">None detected.</p>
        ) : (
          <div className="mt-2 space-y-6">
            {componentsByDir.map(([dir, items]) => (
              <div key={dir}>
                <div className="mb-1 font-mono text-xs text-ink-dim">
                  {dir === "(root)" ? dir : `${dir}/`}
                </div>
                <ul className="divide-y divide-line-faint border-t border-line-faint">
                  {items.map((c, i) => (
                    <li
                      key={`${c.file_path}-${c.name}-${i}`}
                      className="flex flex-wrap items-center gap-2 py-2"
                    >
                      <span className="font-mono text-sm text-ink">{c.name}</span>
                      <span className="text-xs text-ink-dim">
                        {c.type} &middot; {c.language}
                      </span>
                      <CopyableLocation filePath={c.file_path} startLine={c.start_line} endLine={c.end_line} />
                      <OpenInCodeView
                        repoId={repoId}
                        filePath={c.file_path}
                        startLine={c.start_line}
                        endLine={c.end_line}
                      />
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
