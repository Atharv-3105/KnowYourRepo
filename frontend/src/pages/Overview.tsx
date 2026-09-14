import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ApiError, getArchitecture, listRepos, listSymbols } from "../api";
import ErrorState from "../components/ErrorState";
import { SkeletonBlock, SkeletonLine } from "../components/Skeleton";
import StatReading from "../components/StatReading";
import { repoRelativePath, topLevelDir } from "../lib/path";

export default function Overview() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();

  const architectureQuery = useQuery({
    queryKey: ["architecture", repoId],
    queryFn: () => getArchitecture(repoId!),
    enabled: Boolean(repoId),
  });

  // Same query key Home/RepoSwitcher already use - a cache hit here in
  // practice, not a second network round trip, since TanStack Query
  // dedupes by key.
  const reposQuery = useQuery({ queryKey: ["repos"], queryFn: listRepos });
  const repo = reposQuery.data?.find((r) => r.repo_id === repoId);

  // Real directory structure comes from the full symbol index (Brick 13),
  // not from architecture's `components`/`entrypoints` - those are a
  // heuristic subset and would under-represent the repo's actual layout.
  const symbolsQuery = useQuery({
    queryKey: ["symbols", repoId, ""],
    queryFn: () => listSymbols(repoId!),
    enabled: Boolean(repoId),
  });

  const structure = useMemo(() => {
    if (!symbolsQuery.data || !repoId) return [];

    const counts = new Map<string, number>();
    for (const sym of symbolsQuery.data) {
      const dir = topLevelDir(sym.file_path, repoId);
      counts.set(dir, (counts.get(dir) ?? 0) + 1);
    }

    return Array.from(counts.entries()).sort(([a], [b]) => {
      if (a === "(root)") return -1;
      if (b === "(root)") return 1;
      return a.localeCompare(b);
    });
  }, [symbolsQuery.data, repoId]);

  if (architectureQuery.isLoading) {
    return (
      <div className="max-w-3xl space-y-10">
        <div className="space-y-2">
          <SkeletonLine className="w-72" />
          <SkeletonLine className="w-48" />
        </div>
        <SkeletonBlock className="h-16 w-full" />
        <SkeletonBlock className="h-32 w-full" />
      </div>
    );
  }

  if (architectureQuery.isError) {
    return (
      <ErrorState
        message={
          architectureQuery.error instanceof ApiError
            ? architectureQuery.error.message
            : "Failed to load overview."
        }
        onRetry={() => architectureQuery.refetch()}
      />
    );
  }

  const data = architectureQuery.data;
  if (!data || !repoId) return null;

  return (
    <div className="max-w-3xl space-y-10">
      <div>
        <h1 className="font-display text-2xl font-medium text-ink">
          {repo ? repo.repo_url.replace("https://github.com/", "") : repoId}
        </h1>
        {repo && (
          <a
            href={repo.repo_url}
            target="_blank"
            rel="noreferrer"
            className="font-mono text-xs text-ink-dim hover:text-accent"
          >
            {repo.repo_url}
          </a>
        )}
      </div>

      {data.narrative_summary && (
        <p className="max-w-2xl text-sm leading-relaxed text-ink">{data.narrative_summary}</p>
      )}

      {data.languages.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {data.languages.map((lang) => (
            <span key={lang} className="border border-line-faint px-2 py-0.5 font-mono text-xs text-ink-dim">
              {lang}
            </span>
          ))}
        </div>
      )}

      <div className="flex divide-x divide-line-faint border-y border-line-faint">
        <StatReading label="Files" value={data.statistics.file_count} />
        <StatReading label="Symbols" value={data.statistics.symbol_count} />
        <StatReading label="Call edges" value={data.statistics.call_edges} />
      </div>

      <section>
        <h2 className="text-sm text-ink-dim">Repository structure</h2>
        {symbolsQuery.isLoading ? (
          <div className="mt-2 space-y-1.5">
            <SkeletonLine className="w-full" />
            <SkeletonLine className="w-full" />
            <SkeletonLine className="w-2/3" />
          </div>
        ) : structure.length === 0 ? (
          <p className="mt-2 text-sm text-ink-faint">No symbols detected.</p>
        ) : (
          <ul className="mt-2 divide-y divide-line-faint border-t border-line-faint font-mono text-sm">
            {structure.map(([dir, count]) => (
              <li key={dir} className="flex items-center justify-between py-1.5">
                <span className="text-ink">{dir === "(root)" ? dir : `${dir}/`}</span>
                <span className="text-ink-dim">
                  {count} symbol{count === 1 ? "" : "s"}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      {data.reading_path.length > 0 ? (
        <section>
          <h2 className="text-sm text-ink-dim">Where to start reading</h2>
          <ol className="mt-2 space-y-2 font-mono text-sm">
            {data.reading_path.map((step, i) => (
              <li key={`${step.symbol}-${i}`} className="text-ink">
                <div className="flex flex-wrap items-baseline gap-2">
                  <span>{step.symbol}</span>
                  {step.file_path && (
                    <span className="text-xs text-ink-dim">{repoRelativePath(step.file_path, repoId!)}</span>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs text-ink-faint">
                  <span>{step.reason}</span>
                  <button
                    type="button"
                    onClick={() =>
                      navigate(`/repos/${repoId}/chat?prefill=${encodeURIComponent(`What does ${step.symbol} do?`)}`)
                    }
                    className="border border-line-faint px-1.5 py-0.5 text-ink-dim transition-colors hover:border-accent hover:text-ink"
                  >
                    ask about this
                  </button>
                </div>
              </li>
            ))}
          </ol>
        </section>
      ) : (
        data.entrypoints.length > 0 && (
          <section>
            <h2 className="text-sm text-ink-dim">Entrypoints</h2>
            <ul className="mt-2 space-y-1 font-mono text-sm">
              {data.entrypoints.map((ep, i) => (
                <li key={i} className="text-ink">
                  {ep.name} <span className="text-ink-dim">&middot; {repoRelativePath(ep.file_path, repoId!)}</span>
                </li>
              ))}
            </ul>
          </section>
        )
      )}

      {data.concepts.length > 0 && (
        <section>
          <h2 className="text-sm text-ink-dim">Notable concepts</h2>
          <ul className="mt-2 space-y-2 font-mono text-sm">
            {data.concepts.map((concept, i) => (
              <li key={`${concept.term}-${i}`} className="text-ink">
                <div className="flex flex-wrap items-baseline gap-2">
                  <span>{concept.term}</span>
                  <button
                    type="button"
                    onClick={() =>
                      navigate(
                        `/repos/${repoId}/chat?prefill=${encodeURIComponent(`What does "${concept.term}" mean in this codebase?`)}`,
                      )
                    }
                    className="border border-line-faint px-1.5 py-0.5 text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
                  >
                    ask about this
                  </button>
                </div>
                <p className="text-xs text-ink-faint">{concept.explanation}</p>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
