import { Link, NavLink, Outlet, useParams } from "react-router-dom";

const TABS = [
  { to: "architecture", label: "Architecture" },
  { to: "chat", label: "Chat" },
  { to: "graph", label: "Graph" },
];

// Shared shell for every repo-scoped screen: an instrument-panel header
// (repo id as a real readout, not a caption) and tabs styled as panel
// toggles rather than underlined SaaS tabs - content renders via
// <Outlet /> below.
export default function RepoLayout() {
  const { repoId } = useParams<{ repoId: string }>();

  return (
    <div className="min-h-screen">
      <div className="border-b border-line-faint bg-paper-deep">
        <div className="mx-auto max-w-4xl px-6 py-4">
          <Link to="/" className="text-sm text-line-dim hover:text-line">
            &larr; repositories
          </Link>
          <div className="mt-2 font-mono text-xs text-line-dim">
            repo <span className="text-line">{repoId}</span>
          </div>
        </div>

        <nav className="mx-auto flex max-w-4xl gap-1 px-6">
          {TABS.map((tab) => (
            <NavLink
              key={tab.to}
              to={tab.to}
              className={({ isActive }) =>
                `border-t-2 px-4 py-2.5 text-sm ${
                  isActive
                    ? "border-accent bg-paper text-line"
                    : "border-transparent text-line-dim hover:text-line"
                }`
              }
            >
              {tab.label}
            </NavLink>
          ))}
        </nav>
      </div>

      <div className="mx-auto max-w-4xl px-6 py-8">
        <Outlet />
      </div>
    </div>
  );
}
