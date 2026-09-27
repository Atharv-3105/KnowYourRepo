import { lazy, Suspense } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import Home from "./pages/Home";
import IngestionProgress from "./pages/IngestionProgress";
import Overview from "./pages/Overview";
import RepoLayout from "./pages/RepoLayout";
import SymbolExplorer from "./pages/SymbolExplorer";

// The graph (Cytoscape + three.js) and the Chat/Architecture views (Shiki,
// markdown) carry most of the bundle weight - load them on first visit
// instead of making the home page download them.
const GraphView = lazy(() => import("./pages/GraphView"));
const Chat = lazy(() => import("./pages/Chat"));
const Architecture = lazy(() => import("./pages/Architecture"));

function RouteFallback() {
  return <div className="p-8 text-sm text-ink-dim">Loading…</div>;
}

function App() {
  return (
    <Suspense fallback={<RouteFallback />}>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/repos/:repoId/status" element={<IngestionProgress />} />
        <Route path="/repos/:repoId" element={<RepoLayout />}>
          <Route index element={<Navigate to="overview" replace />} />
          <Route path="overview" element={<Overview />} />
          <Route path="chat" element={<Chat />} />
          <Route path="architecture" element={<Architecture />} />
          <Route path="symbols" element={<SymbolExplorer />} />
          <Route path="graph" element={<GraphView />} />
        </Route>
      </Routes>
    </Suspense>
  );
}

export default App;
