import { Boxes, Braces, Compass, MessageSquare, Workflow } from "lucide-react";
import { motion } from "motion/react";
import { lazy, Suspense, useEffect, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useParams } from "react-router-dom";
import { CodeViewProvider } from "../components/CodeViewProvider";
import CommandPalette from "../components/CommandPalette";
import RepoSwitcher from "../components/RepoSwitcher";

// Shiki plus its curated language set is a genuinely large dependency
// (confirmed directly - eagerly importing it grew the main bundle by
// several MB). Lazy-loading the whole panel means that weight lives in
// its own chunk, fetched once RepoLayout mounts rather than blocking the
// app's initial bundle - most visits to a repo never open a file.
const CodeViewPanel = lazy(() => import("../components/CodeViewPanel"));

const NAV_ITEMS = [
  { to: "overview", label: "Overview", icon: Compass },
  { to: "chat", label: "Chat", icon: MessageSquare },
  { to: "architecture", label: "Architecture", icon: Boxes },
  { to: "symbols", label: "Symbols", icon: Braces },
  { to: "graph", label: "Graph", icon: Workflow },
];

// Persistent app shell: a top bar (product name, repo switcher, search
// trigger) and a left nav whose active item is tracked by a single
// motion-animated indicator that slides between items on navigation,
// rather than each item snapping its own highlight on and off.
export default function RepoLayout() {
  const { repoId } = useParams<{ repoId: string }>();
  const location = useLocation();
  const [paletteOpen, setPaletteOpen] = useState(false);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((prev) => !prev);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  if (!repoId) return null;

  return (
    <CodeViewProvider>
      <div className="flex h-screen flex-col">
        <header className="flex min-h-12 shrink-0 flex-wrap items-center justify-between gap-2 border-b border-line-faint bg-page-deep px-4 py-2">
          <div className="flex items-center gap-4">
            <Link to="/" className="font-mono text-sm font-semibold text-ink">
              KnowYourRepo
            </Link>
            <RepoSwitcher currentRepoId={repoId} />
          </div>

          <button
            type="button"
            onClick={() => setPaletteOpen(true)}
            className="flex items-center gap-1.5 border border-line-faint px-2 py-1 text-xs text-ink-dim hover:text-ink"
          >
            <span>Search</span>
            <kbd className="font-mono text-[10px] text-ink-faint">&#8984;K</kbd>
          </button>
        </header>

        <div className="flex flex-1 overflow-hidden">
          {/* Icon-only rail below md (768px): a persistent 192px-wide
              labeled sidebar leaves too little width for real content on
              a phone-width viewport, and there's no drawer/overlay
              mechanism here to hide it behind - collapsing to icons keeps
              every destination reachable without introducing a second,
              unverified interaction pattern (see this brick's doc on why
              a hamburger drawer wasn't built without a way to visually
              test it). */}
          <aside className="w-12 shrink-0 overflow-y-auto border-r border-line-faint bg-page-deep md:w-48">
            <nav className="space-y-0.5 px-2 py-3">
              {NAV_ITEMS.map((item) => {
                const isActive = location.pathname.endsWith(`/${item.to}`);
                const Icon = item.icon;

                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    title={item.label}
                    className="relative flex items-center justify-center gap-2.5 px-3 py-2 text-sm text-ink-dim data-[active=true]:text-ink md:justify-start"
                    data-active={isActive}
                  >
                    {isActive && (
                      <motion.div
                        layoutId="nav-active-indicator"
                        className="absolute inset-0 border-l-2 border-accent bg-page"
                        transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
                      />
                    )}
                    <Icon size={15} className="relative z-10 shrink-0" />
                    <span className="relative z-10 hidden md:inline">{item.label}</span>
                  </NavLink>
                );
              })}
            </nav>
          </aside>

          <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8 md:py-8">
            <Outlet />
          </main>
        </div>

        <CommandPalette repoId={repoId} open={paletteOpen} onOpenChange={setPaletteOpen} />
        <Suspense fallback={null}>
          <CodeViewPanel repoId={repoId} />
        </Suspense>
      </div>
    </CodeViewProvider>
  );
}
