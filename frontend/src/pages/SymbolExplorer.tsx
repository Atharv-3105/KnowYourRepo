import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ApiError, getBoundedGraph, listSymbols } from "../api";
import CopyableLocation from "../components/CopyableLocation";
import ErrorState from "../components/ErrorState";
import OpenInCodeView from "../components/OpenInCodeView";
import { SkeletonLine } from "../components/Skeleton";
import { useDebouncedValue } from "../lib/useDebouncedValue";

// Searchable index over the repo's real, complete symbol set (Brick 13) -
// not Architecture's heuristic components subset. Selecting a row expands
// it inline to show what it calls (via the bounded graph endpoint at
// depth=1). "Called by" isn't shown: Brick 3's bounded-graph endpoint only
// traverses outgoing edges by design (see that brick's doc) - there's no
// real backend data for incoming callers to show here, so this
// deliberately doesn't fake one.
export default function SymbolExplorer() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const debouncedQuery = useDebouncedValue(query, 250);

  const symbolsQuery = useQuery({
    queryKey: ["symbols", repoId, debouncedQuery],
    queryFn: () => listSymbols(repoId!, debouncedQuery),
    enabled: Boolean(repoId),
  });

  const types = useMemo(() => {
    const set = new Set((symbolsQuery.data ?? []).map((s) => s.type));
    return Array.from(set).sort();
  }, [symbolsQuery.data]);

  const filtered = useMemo(() => {
    if (!symbolsQuery.data) return [];
    return typeFilter ? symbolsQuery.data.filter((s) => s.type === typeFilter) : symbolsQuery.data;
  }, [symbolsQuery.data, typeFilter]);

  const callsQuery = useQuery({
    queryKey: ["graph", repoId, selected, 1],
    queryFn: () => getBoundedGraph(repoId!, selected!, 1),
    enabled: Boolean(repoId) && Boolean(selected),
  });

  return (
    <div>
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search symbols..."
          className="flex-1 border border-line-faint bg-page-deep px-3 py-2 font-mono text-sm text-ink placeholder:text-ink-dim"
        />
        {types.length > 1 && (
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="border border-line-faint bg-page-deep px-2 py-2 text-sm text-ink-dim"
          >
            <option value="">all kinds</option>
            {types.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        )}
      </div>

      {symbolsQuery.isLoading && (
        <ul className="mt-4 divide-y divide-line-faint border-t border-line-faint">
          {[0, 1, 2, 3, 4].map((i) => (
            <li key={i} className="py-2">
              <SkeletonLine className="w-2/3" />
            </li>
          ))}
        </ul>
      )}

      {symbolsQuery.isError && (
        <div className="mt-4">
          <ErrorState
            message={
              symbolsQuery.error instanceof ApiError ? symbolsQuery.error.message : "Failed to load symbols."
            }
            onRetry={() => symbolsQuery.refetch()}
          />
        </div>
      )}

      {filtered.length === 0 && !symbolsQuery.isLoading && !symbolsQuery.isError && (
        <p className="mt-4 text-sm text-ink-faint">No symbols found.</p>
      )}

      <ul className="mt-4 divide-y divide-line-faint border-t border-line-faint">
        {filtered.map((sym, i) => {
          const isOpen = selected === sym.name;
          return (
            <li key={`${sym.name}-${i}`}>
              <div className="flex w-full flex-wrap items-center gap-2 py-2">
                <button
                  type="button"
                  onClick={() => setSelected(isOpen ? null : sym.name)}
                  className="flex items-center gap-2 text-left"
                >
                  <span className="font-mono text-sm text-ink">{sym.name}</span>
                  <span className="text-xs text-ink-dim">
                    {sym.type} &middot; {sym.language}
                  </span>
                </button>
                <CopyableLocation filePath={sym.file_path} startLine={sym.start_line} endLine={sym.end_line} />
                <OpenInCodeView
                  repoId={repoId!}
                  filePath={sym.file_path}
                  startLine={sym.start_line}
                  endLine={sym.end_line}
                />
              </div>

              {isOpen && (
                <div className="mb-3 ml-2 border-l-2 border-line-faint pl-4 text-sm">
                  {callsQuery.isLoading && <SkeletonLine className="w-40" />}
                  {callsQuery.data && callsQuery.data.edges.length === 0 && (
                    <p className="text-ink-faint">No outgoing calls detected.</p>
                  )}
                  {callsQuery.data && callsQuery.data.edges.length > 0 && (
                    <div>
                      <div className="mb-1 text-xs text-ink-dim">Calls</div>
                      <div className="flex flex-wrap gap-1.5">
                        {callsQuery.data.edges
                          .filter((e) => e.caller === sym.name)
                          .map((e, ei) => (
                            <span key={ei} className="border border-line-faint px-1.5 py-0.5 font-mono text-xs text-ink-dim">
                              {e.callee}
                            </span>
                          ))}
                      </div>
                    </div>
                  )}
                  <button
                    type="button"
                    onClick={() => navigate(`/repos/${repoId}/graph?symbol=${encodeURIComponent(sym.name)}`)}
                    className="mt-2 border border-line-faint px-1.5 py-0.5 text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
                  >
                    view in graph
                  </button>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
