import { useQuery } from "@tanstack/react-query";
import cytoscape from "cytoscape";
import dagre from "cytoscape-dagre";
import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { ApiError, getArchitecture, getBoundedGraph } from "../api";
import type { GraphNode } from "../api";
import CopyableLocation from "../components/CopyableLocation";
import ErrorState from "../components/ErrorState";
import { SkeletonBlock } from "../components/Skeleton";
import OpenInCodeView from "../components/OpenInCodeView";
import { duration } from "../lib/motion";

cytoscape.use(dagre);

const DEFAULT_DEPTH = 2;
// Cytoscape's own transition-duration is a plain number of seconds, so
// duration.base (already in seconds, lib/motion.ts) applies directly with
// no unit conversion. Its timing-function type only accepts named curves,
// not arbitrary cubic-bezier arrays - "ease-in-out" is the closest built-in
// match to the shared easeInOut curve, which lib/motion.ts names for
// exactly this use (symmetric fade, no overshoot).
//
// The graph canvas is Cytoscape's own render surface, not React DOM - it's
// covered by neither index.css's prefers-reduced-motion rule (real CSS
// transitions only) nor main.tsx's MotionConfig (motion's own components
// only), so it needs its own explicit check.
const prefersReducedMotion = () =>
  typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

export default function GraphView() {
  const { repoId } = useParams<{ repoId: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<cytoscape.Core | null>(null);

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

  const elements = useMemo(() => {
    if (!graphQuery.data) return [];
    const nodeEls = graphQuery.data.nodes.map((n) => ({
      data: { id: n.symbol, label: n.symbol, filePath: n.file_path ?? "" },
    }));
    const edgeEls = graphQuery.data.edges.map((e) => ({
      data: { id: `${e.caller}->${e.callee}`, source: e.caller, target: e.callee },
    }));
    return [...nodeEls, ...edgeEls];
  }, [graphQuery.data]);

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

  useEffect(() => {
    if (!containerRef.current) return;

    // Checked per cy instance (elements/rootSymbol change), not just once
    // at module load - cheap, and means a session that starts before the
    // OS setting changes still picks it up on the next graph rebuild.
    const focusTransition = prefersReducedMotion() ? 0 : duration.base;
    const focusEasing = "ease-in-out" as const;

    const cy = cytoscape({
      container: containerRef.current,
      elements,
      style: [
        {
          selector: "node",
          style: {
            label: "data(label)",
            "font-family": '"JetBrains Mono", monospace',
            "font-size": 10,
            color: "#1a1d24",
            "background-color": "#6b6f7a",
            "border-width": 0,
            width: 22,
            height: 22,
            "text-valign": "bottom",
            "text-margin-y": 6,
            "transition-property": "opacity",
            "transition-duration": focusTransition,
            "transition-timing-function": focusEasing,
          },
        },
        {
          // Rust (--color-tool-graph), not the primary accent blue - the
          // root-symbol emphasis on this page is specifically the graph
          // tool's own output, so it takes the graph tool's color rather
          // than the app-wide interactive accent (see this brick's doc).
          selector: `node[id = "${rootSymbol}"]`,
          style: { "background-color": "#b5502c", width: 30, height: 30 },
        },
        {
          selector: "edge",
          style: {
            width: 1.25,
            "line-color": "#9a9d9d",
            "target-arrow-color": "#9a9d9d",
            "target-arrow-shape": "triangle",
            "arrow-scale": 0.8,
            "curve-style": "bezier",
            "transition-property": "opacity",
            "transition-duration": focusTransition,
            "transition-timing-function": focusEasing,
          },
        },
        {
          // Focus mode: elements not connected to the selected node fade
          // out in place (opacity only) - deliberately not a re-layout,
          // which would disorient anyone tracking the graph's shape.
          selector: ".dimmed",
          style: { opacity: 0.15 },
        },
      ],
      layout: { name: "dagre" } as cytoscape.LayoutOptions,
    });

    cy.on("tap", "node", (evt) => {
      const node = evt.target;
      const data = node.data();
      setSelectedNode({ symbol: data.id, file_path: data.filePath || undefined });

      const neighborhood = node.closedNeighborhood();
      cy.elements().difference(neighborhood).addClass("dimmed");
      neighborhood.removeClass("dimmed");
    });

    // Tapping empty canvas (not an element) clears focus mode and
    // deselects - evt.target is the core itself only for background taps,
    // since node taps are handled by the delegate listener above.
    cy.on("tap", (evt) => {
      if (evt.target === cy) {
        cy.elements().removeClass("dimmed");
        setSelectedNode(null);
      }
    });

    cyRef.current = cy;

    return () => {
      cy.destroy();
      cyRef.current = null;
    };
  }, [elements, rootSymbol]);

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
          {/* Always mounted once a root symbol exists (not conditionally
              swapped with the skeleton below) - Cytoscape attaches to this
              exact DOM node in the effect above, keyed on containerRef;
              unmounting and remounting it on every loading transition
              would tear down and rebuild that attachment for no reason. */}
          <div className="relative h-[420px] w-full">
            {/* Top rule in the graph tool's own accent, not a decorative
                border - this canvas IS that tool's output, the same
                reasoning behind Chat's per-tool badge coloring (Brick 25). */}
            <div
              ref={containerRef}
              className="h-full w-full border border-line-faint border-t-2 border-t-tool-graph bg-page-deep"
            />
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
