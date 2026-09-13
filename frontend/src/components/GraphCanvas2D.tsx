import cytoscape from "cytoscape";
import dagre from "cytoscape-dagre";
import { useEffect, useRef } from "react";
import type { GraphNode } from "../api";
import { duration } from "../lib/motion";

cytoscape.use(dagre);

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

export interface GraphCanvasProps {
  nodes: GraphNode[];
  edges: { caller: string; callee: string }[];
  rootSymbol: string;
  onSelectNode: (node: GraphNode | null) => void;
}

export default function GraphCanvas2D({ nodes, edges, rootSymbol, onSelectNode }: GraphCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<cytoscape.Core | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    const elements = [
      ...nodes.map((n) => ({
        data: { id: n.symbol, label: n.symbol, filePath: n.file_path ?? "" },
      })),
      ...edges.map((e) => ({
        data: { id: `${e.caller}->${e.callee}`, source: e.caller, target: e.callee },
      })),
    ];

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
      onSelectNode({ symbol: data.id, file_path: data.filePath || undefined });

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
        onSelectNode(null);
      }
    });

    cyRef.current = cy;

    return () => {
      cy.destroy();
      cyRef.current = null;
    };
  }, [nodes, edges, rootSymbol, onSelectNode]);

  return <div ref={containerRef} className="h-full w-full" />;
}
