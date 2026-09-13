import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { ApiError, getArchitecture, getBoundedGraph } from "../api";
import type { GraphNode } from "../api";
import CopyableLocation from "../components/CopyableLocation";
import ErrorState from "../components/ErrorState";
import GraphCanvas2D from "../components/GraphCanvas2D";
import GraphCanvas3D from "../components/GraphCanvas3D";
import { SkeletonBlock } from "../components/Skeleton";
import OpenInCodeView from "../components/OpenInCodeView";

const DEFAULT_DEPTH = 2;

export default function GraphView() {
  const { repoId } = useParams<{ repoId: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [renderMode, setRenderMode] = useState<"2d" | "3d">("2d");

  // Priority: a symbol the user explicitly picked in this session > a
  // ?symbol= param (arrived here via a "View in graph" link from a Chat
  // citation, Brick 10) > the repo's first entrypoint. All three are
  // derived during render, not synced via an effect - see Decisions Made
  // in this brick's doc for why that matters.
  const [manualRootSymbol, setManualRootSymbol] = useState("");
  const [depth, setDepth] = useState(DEFAULT_DEPTH);
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null);

  // Entrypoints (Brick 7's data) seed the symbol picker - a graph needs
  // somewhere to start from, and entrypoints are the natural "explore from
  // here" candidates for a call graph.
  const architectureQuery = useQuery({
    queryKey: ["architecture", repoId],
    queryFn: () => getArchitecture(repoId!),
    enabled: Boolean(repoId),
  });

  const rootSymbol =
    manualRootSymbol ||
    searchParams.get("symbol") ||
    architectureQuery.data?.entrypoints[0]?.name ||
    "";

  const graphQuery = useQuery({
    queryKey: ["graph", repoId, rootSymbol, depth],
    queryFn: () => getBoundedGraph(repoId!, rootSymbol, depth),
    enabled: Boolean(repoId) && Boolean(rootSymbol),
  });

  // Callers/callees for the selected node, within the currently loaded
  // bounded graph only (not an exhaustive repo-wide lookup - the backend's
  // /graph endpoint is intentionally depth-bounded, see Brick 3's doc).
  // Clicking a name re-centers the graph on it, reusing the same
  // manualRootSymbol state the symbol picker above already writes to.
  const callers = useMemo(() => {
    if (!selectedNode || !graphQuery.data) return [];
    return graphQuery.data.edges.filter((e) => e.callee === selectedNode.symbol).map((e) => e.caller);
  }, [selectedNode, graphQuery.data]);

  const callees = useMemo(() => {
    if (!selectedNode || !graphQuery.data) return [];
    return graphQuery.data.edges.filter((e) => e.caller === selectedNode.symbol).map((e) => e.callee);
  }, [selectedNode, graphQuery.data]);

  const focusOn = (symbol: string) => setManualRootSymbol(symbol);

  return (
    <div>
      <div className="flex flex-wrap items-end gap-4">
        <label className="text-sm">
          <span className="mb-1 block text-xs text-ink-dim">Root symbol</span>
          {architectureQuery.data && architectureQuery.data.entrypoints.length > 0 ? (
            <select
              value={rootSymbol}
              onChange={(e) => setManualRootSymbol(e.target.value)}
              className="border border-line-faint bg-page-deep px-2.5 py-1.5 font-mono text-sm text-ink"
            >
              {architectureQuery.data.entrypoints.map((ep) => (
                <option key={ep.name} value={ep.name}>
                  {ep.name}
                </option>
              ))}
            </select>
          ) : (
            <input
              type="text"
              value={rootSymbol}
              onChange={(e) => setManualRootSymbol(e.target.value)}
              placeholder="symbol name"
              className="border border-line-faint bg-page-deep px-2.5 py-1.5 font-mono text-sm text-ink placeholder:text-ink-dim"
            />
          )}
        </label>

        <label className="text-sm">
          <span className="mb-1 block text-xs text-ink-dim">Depth</span>
          <input
            type="number"
            min={1}
            max={5}
            value={depth}
            onChange={(e) => setDepth(Number(e.target.value))}
            className="w-16 border border-line-faint bg-page-deep px-2.5 py-1.5 font-mono text-sm text-ink"
          />
        </label>

        <div className="text-sm">
          <span className="mb-1 block text-xs text-ink-dim">View</span>
          <div className="flex border border-line-faint">
            {(["2d", "3d"] as const).map((mode) => (
              <button
                key={mode}
                type="button"
                onClick={() => setRenderMode(mode)}
                aria-pressed={renderMode === mode}
                className={`px-2.5 py-1.5 font-mono text-sm uppercase transition-colors ${
                  renderMode === mode
                    ? "bg-tool-graph text-page-deep"
                    : "text-ink-dim hover:text-ink"
                }`}
              >
                {mode}
              </button>
            ))}
          </div>
        </div>
      </div>

      {architectureQuery.isError && (
        <div className="mt-4">
          <ErrorState
            message={
              architectureQuery.error instanceof ApiError
                ? architectureQuery.error.message
                : "Failed to load repository architecture."
            }
            onRetry={() => architectureQuery.refetch()}
          />
        </div>
      )}

      {!rootSymbol && !architectureQuery.isError && (
        <p className="mt-4 text-sm text-ink-dim">
          No entrypoints detected for this repo - type a symbol name above to explore its call
          graph.
        </p>
      )}

      {graphQuery.isError && (
        <div className="mt-4">
          <ErrorState
            message={graphQuery.error instanceof ApiError ? graphQuery.error.message : "Failed to load graph."}
            onRetry={() => graphQuery.refetch()}
          />
        </div>
      )}

      {graphQuery.data?.truncated && (
        <p className="mt-4 text-xs text-tool-graph">
          This graph was too large to show in full - showing a partial view.
        </p>
      )}

      {rootSymbol && (
        <figure className="mt-4">
          <div className="relative h-[420px] w-full">
            {/* Top rule in the graph tool's own accent, not a decorative
                border - this canvas IS that tool's output, the same
                reasoning behind Chat's per-tool badge coloring (Brick 25). */}
            <div className="h-full w-full border border-line-faint border-t-2 border-t-tool-graph bg-page-deep">
              {graphQuery.data &&
                (renderMode === "2d" ? (
                  <GraphCanvas2D
                    nodes={graphQuery.data.nodes}
                    edges={graphQuery.data.edges}
                    rootSymbol={rootSymbol}
                    onSelectNode={setSelectedNode}
                  />
                ) : (
                  <GraphCanvas3D
                    nodes={graphQuery.data.nodes}
                    edges={graphQuery.data.edges}
                    rootSymbol={rootSymbol}
                    onSelectNode={setSelectedNode}
                  />
                ))}
            </div>
            {graphQuery.isLoading && (
              <SkeletonBlock className="absolute inset-0 border border-line-faint" />
            )}
          </div>
          <figcaption className="mt-1.5 font-mono text-xs text-ink-faint">
            Call graph rooted at <span className="text-ink-dim">{rootSymbol}</span>, depth {depth}
          </figcaption>
        </figure>
      )}

      {selectedNode && (
        <div className="mt-3 border border-line-faint bg-page-deep px-3 py-2.5 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-ink">{selectedNode.symbol}</span>
            {selectedNode.file_path ? (
              <>
                <CopyableLocation filePath={selectedNode.file_path} />
                <OpenInCodeView repoId={repoId!} filePath={selectedNode.file_path} />
              </>
            ) : (
              <span className="text-xs text-ink-faint">no location data for this symbol</span>
            )}
            <button
              type="button"
              onClick={() =>
                navigate(`/repos/${repoId}/chat?prefill=${encodeURIComponent(`What does ${selectedNode.symbol} do?`)}`)
              }
              className="border border-line-faint px-1.5 py-0.5 text-xs text-ink-dim transition-colors hover:border-accent hover:text-ink"
            >
              ask about this
            </button>
          </div>

          <div className="mt-2.5 grid grid-cols-1 gap-3 border-t border-line-faint pt-2.5 sm:grid-cols-2">
            <div>
              <div className="mb-1 text-xs text-ink-dim">
                Called by {callers.length > 0 ? `(${callers.length})` : ""}
              </div>
              {callers.length === 0 ? (
                <p className="text-xs text-ink-faint">
                  No callers within this graph's depth bound.
                </p>
              ) : (
                <ul className="space-y-1">
                  {callers.map((c) => (
                    <li key={c}>
                      <button
                        type="button"
                        onClick={() => focusOn(c)}
                        className="font-mono text-xs text-ink-dim hover:text-ink hover:underline"
                      >
                        {c}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div>
              <div className="mb-1 text-xs text-ink-dim">
                Calls {callees.length > 0 ? `(${callees.length})` : ""}
              </div>
              {callees.length === 0 ? (
                <p className="text-xs text-ink-faint">
                  No callees within this graph's depth bound.
                </p>
              ) : (
                <ul className="space-y-1">
                  {callees.map((c) => (
                    <li key={c}>
                      <button
                        type="button"
                        onClick={() => focusOn(c)}
                        className="font-mono text-xs text-ink-dim hover:text-ink hover:underline"
                      >
                        {c}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
