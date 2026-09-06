import { useQuery } from "@tanstack/react-query";
import cytoscape from "cytoscape";
import dagre from "cytoscape-dagre";
import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { ApiError, getArchitecture, getBoundedGraph } from "../api";
import type { GraphNode } from "../api";
import CopyableLocation from "../components/CopyableLocation";

cytoscape.use(dagre);

const DEFAULT_DEPTH = 2;

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

  useEffect(() => {
    if (!containerRef.current) return;

    const cy = cytoscape({
      container: containerRef.current,
      elements,
      style: [
        {
          selector: "node",
          style: {
            label: "data(label)",
            "font-family": '"IBM Plex Mono", monospace',
            "font-size": 10,
            color: "#eaf2f8",
            "background-color": "#8fafc7",
            "border-width": 0,
            width: 22,
            height: 22,
            "text-valign": "bottom",
            "text-margin-y": 6,
          },
        },
        {
          selector: `node[id = "${rootSymbol}"]`,
          style: { "background-color": "#f2a65a", width: 30, height: 30 },
        },
        {
          selector: "edge",
          style: {
            width: 1.25,
            "line-color": "#3d5975",
            "target-arrow-color": "#3d5975",
            "target-arrow-shape": "triangle",
            "arrow-scale": 0.8,
            "curve-style": "bezier",
          },
        },
      ],
      layout: { name: "dagre" } as cytoscape.LayoutOptions,
    });

    cy.on("tap", "node", (evt) => {
      const data = evt.target.data();
      setSelectedNode({ symbol: data.id, file_path: data.filePath || undefined });
    });

    cyRef.current = cy;

    return () => {
      cy.destroy();
      cyRef.current = null;
    };
  }, [elements, rootSymbol]);

  return (
    <div>
      <div className="flex flex-wrap items-end gap-4">
        <label className="text-sm">
          <span className="mb-1 block text-xs text-line-dim">Root symbol</span>
          {architectureQuery.data && architectureQuery.data.entrypoints.length > 0 ? (
            <select
              value={rootSymbol}
              onChange={(e) => setManualRootSymbol(e.target.value)}
              className="border border-line-faint bg-paper-deep px-2.5 py-1.5 font-mono text-sm text-line"
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
              className="border border-line-faint bg-paper-deep px-2.5 py-1.5 font-mono text-sm text-line placeholder:text-line-dim"
            />
          )}
        </label>

        <label className="text-sm">
          <span className="mb-1 block text-xs text-line-dim">Depth</span>
          <input
            type="number"
            min={1}
            max={5}
            value={depth}
            onChange={(e) => setDepth(Number(e.target.value))}
            className="w-16 border border-line-faint bg-paper-deep px-2.5 py-1.5 font-mono text-sm text-line"
          />
        </label>
      </div>

      {!rootSymbol && (
        <p className="mt-4 text-sm text-line-dim">
          No entrypoints detected for this repo - type a symbol name above to explore its call
          graph.
        </p>
      )}

      {graphQuery.isError && (
        <p className="mt-4 text-sm text-danger">
          {graphQuery.error instanceof ApiError ? graphQuery.error.message : "Failed to load graph."}
        </p>
      )}

      {graphQuery.data?.truncated && (
        <p className="mt-4 text-xs text-accent">
          This graph was too large to show in full - showing a partial view.
        </p>
      )}

      {rootSymbol && (
        <div
          ref={containerRef}
          className="mt-4 h-[420px] w-full border border-line-faint bg-paper-deep"
          style={{
            backgroundImage:
              "linear-gradient(#20395a 1px, transparent 1px), linear-gradient(90deg, #20395a 1px, transparent 1px)",
            backgroundSize: "24px 24px",
          }}
        />
      )}

      {selectedNode && (
        <div className="mt-3 flex items-center gap-2 border border-line-faint bg-paper-deep px-3 py-2 text-sm">
          <span className="font-mono text-line">{selectedNode.symbol}</span>
          {selectedNode.file_path ? (
            <CopyableLocation filePath={selectedNode.file_path} />
          ) : (
            <span className="text-xs text-line-faint">no location data for this symbol</span>
          )}
          <button
            type="button"
            onClick={() =>
              navigate(`/repos/${repoId}/chat?prefill=${encodeURIComponent(`What does ${selectedNode.symbol} do?`)}`)
            }
            className="border border-line-faint px-1.5 py-0.5 text-xs text-line-dim transition-colors hover:border-accent hover:text-line"
          >
            ask about this
          </button>
        </div>
      )}
    </div>
  );
}
