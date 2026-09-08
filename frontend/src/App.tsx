import { Navigate, Route, Routes } from "react-router-dom";
import Architecture from "./pages/Architecture";
import Chat from "./pages/Chat";
import GraphView from "./pages/GraphView";
import Home from "./pages/Home";
import IngestionProgress from "./pages/IngestionProgress";
import Overview from "./pages/Overview";
import RepoLayout from "./pages/RepoLayout";
import SymbolExplorer from "./pages/SymbolExplorer";

function App() {
  return (
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
  );
}

export default App;
