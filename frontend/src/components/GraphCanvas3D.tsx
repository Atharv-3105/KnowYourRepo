import ForceGraph3D, { type ForceGraphMethods, type NodeObject } from "react-force-graph-3d";
import { useEffect, useMemo, useRef, useState } from "react";
import * as THREE from "three";
import SpriteText from "three-spritetext";
import type { GraphNode } from "../api";
import type { GraphCanvasProps } from "./GraphCanvas2D";

// Same tokens as GraphCanvas2D - kept in sync by hand, same constraint
// documented for the Cytoscape canvas: neither WebGL nor Cytoscape's own
// canvas can read CSS custom properties, so both hardcode these literals.
const ROOT_COLOR = "#b5502c"; // --color-tool-graph
const NODE_COLOR = "#6b6f7a";
const EDGE_COLOR = "#9a9d9d";
const LABEL_COLOR = "#1a1d24"; // matches GraphCanvas2D's node label color
const SCENE_BG = "#f1f0eb"; // --color-page-deep - the light theme's canvas surface
const DIMMED_OPACITY = 0.15;

const prefersReducedMotion = () =>
  typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

interface Node3D {
  id: string;
  filePath: string;
}

interface Link3D {
  source: string;
  target: string;
}

export default function GraphCanvas3D({ nodes, edges, rootSymbol, onSelectNode }: GraphCanvasProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<ForceGraphMethods<Node3D, Link3D>>(undefined);
  const [width, setWidth] = useState(0);

  // Read inside the accessor closures below via refs, not React state -
  // the closures are handed to the engine once and only re-run on demand
  // (via fgRef.refresh(), see the click handler), so they need the latest
  // value without forcing GraphCanvas3D itself to re-render on every hover.
  const selectedIdRef = useRef<string | null>(null);
  const neighborsRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    if (!wrapperRef.current) return;
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) setWidth(entry.contentRect.width);
    });
    observer.observe(wrapperRef.current);
    return () => observer.disconnect();
  }, []);

  const graphData = useMemo(
    () => ({
      nodes: nodes.map((n) => ({ id: n.symbol, filePath: n.file_path ?? "" })),
      links: edges.map((e) => ({ source: e.caller, target: e.callee })),
    }),
    [nodes, edges],
  );

  // Resets focus state whenever the underlying graph changes (new root
  // symbol/depth) - a stale selection referring to a node that may no
  // longer exist would otherwise leave every node dimmed.
  useEffect(() => {
    selectedIdRef.current = null;
    neighborsRef.current = new Set();
    fgRef.current?.refresh();
  }, [graphData]);

  const isDimmed = (id: string) =>
    selectedIdRef.current !== null && id !== selectedIdRef.current && !neighborsRef.current.has(id);

  const selectNode = (node: NodeObject<Node3D> | null) => {
    if (!node?.id) {
      selectedIdRef.current = null;
      neighborsRef.current = new Set();
      onSelectNode(null);
    } else {
      const id = String(node.id);
      selectedIdRef.current = id;
      const neighbors = new Set<string>();
      for (const link of graphData.links) {
        if (link.source === id) neighbors.add(link.target);
        if (link.target === id) neighbors.add(link.source);
      }
      neighborsRef.current = neighbors;
      onSelectNode({ symbol: id, file_path: node.filePath || undefined } satisfies GraphNode);
    }
    // nodeThreeObject/linkMaterial are cached per element by the underlying
    // engine and aren't re-invoked just because these closures reference
    // new ref values - refresh() is the documented way to force every
    // element's visuals to recompute from the accessors below.
    fgRef.current?.refresh();
  };

  return (
    <div ref={wrapperRef} className="h-full w-full">
      {width > 0 && (
        <ForceGraph3D<Node3D, Link3D>
          ref={fgRef}
          graphData={graphData}
          width={width}
          height={420}
          backgroundColor={SCENE_BG}
          nodeRelSize={5}
          nodeThreeObject={(node) => {
            const group = new THREE.Group();
            const dimmed = isDimmed(node.id);
            const isRoot = node.id === rootSymbol;

            const sphere = new THREE.Mesh(
              new THREE.SphereGeometry(isRoot ? 5 : 3.5),
              new THREE.MeshLambertMaterial({
                color: isRoot ? ROOT_COLOR : NODE_COLOR,
                transparent: true,
                opacity: dimmed ? DIMMED_OPACITY : 1,
              }),
            );
            group.add(sphere);

            const label = new SpriteText(node.id);
            label.color = dimmed ? `rgba(26, 29, 36, ${DIMMED_OPACITY})` : LABEL_COLOR;
            label.textHeight = 3;
            label.position.set(0, isRoot ? 8 : 6.5, 0);
            group.add(label);

            return group;
          }}
          linkMaterial={(link) => {
            const dimmed = isDimmed(String(link.source)) || isDimmed(String(link.target));
            return new THREE.LineBasicMaterial({
              color: EDGE_COLOR,
              transparent: true,
              opacity: dimmed ? DIMMED_OPACITY : 0.6,
            });
          }}
          linkDirectionalArrowLength={3}
          linkDirectionalArrowRelPos={1}
          linkWidth={0.5}
          cooldownTicks={prefersReducedMotion() ? 0 : undefined}
          warmupTicks={prefersReducedMotion() ? 100 : 0}
          onNodeClick={(node) => selectNode(node)}
          onBackgroundClick={() => selectNode(null)}
          showNavInfo={false}
        />
      )}
    </div>
  );
}
