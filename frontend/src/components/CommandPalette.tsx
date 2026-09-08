import { useQuery } from "@tanstack/react-query";
import { Command } from "cmdk";
import { Boxes, Braces, Compass, MessageSquare, Workflow } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { listRepos, listSymbols, search } from "../api";
import { useDebouncedValue } from "../lib/useDebouncedValue";
import { repoRelativePath } from "../lib/path";
import { Dialog } from "./ui/Dialog";

const NAV_ITEMS = [
  { to: "overview", label: "Overview", icon: Compass },
  { to: "chat", label: "Chat", icon: MessageSquare },
  { to: "architecture", label: "Architecture", icon: Boxes },
  { to: "symbols", label: "Symbols", icon: Braces },
  { to: "graph", label: "Graph", icon: Workflow },
];

interface CommandPaletteProps {
  repoId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// ⌘K palette: navigate within the current repo, switch repos, jump to a
// symbol, or run real semantic search - all against real, already-existing
// endpoints (GET /repos, GET /symbols/:repoID, POST /search). Filtering
// is fully manual (shouldFilter=false): static items via a plain substring
// check, symbol/search results are already relevance-ordered server-side,
// so a second client-side fuzzy pass over them would just add noise.
export default function CommandPalette({ repoId, open, onOpenChange }: CommandPaletteProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const debouncedQuery = useDebouncedValue(query, 250);

  const reposQuery = useQuery({ queryKey: ["repos"], queryFn: listRepos });

  const symbolResults = useQuery({
    queryKey: ["symbols", repoId, debouncedQuery],
    queryFn: () => listSymbols(repoId, debouncedQuery),
    enabled: open && debouncedQuery.length >= 2,
  });

  const searchResults = useQuery({
    queryKey: ["search", repoId, debouncedQuery],
    queryFn: () => search(repoId, debouncedQuery),
    enabled: open && debouncedQuery.length >= 3,
  });

  const go = (path: string) => {
    onOpenChange(false);
    setQuery("");
    navigate(path);
  };

  const lowerQuery = query.toLowerCase();
  const matchingNav = NAV_ITEMS.filter((item) => item.label.toLowerCase().includes(lowerQuery));
  const matchingRepos = (reposQuery.data ?? []).filter((r) =>
    r.repo_url.toLowerCase().includes(lowerQuery),
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange} srTitle="Command palette">
      <Command shouldFilter={false} className="flex max-h-[70vh] flex-col">
        <Command.Input
          value={query}
          onValueChange={setQuery}
          autoFocus
          placeholder="Search or jump to..."
          className="border-b border-line-faint bg-transparent px-4 py-3 text-sm text-ink outline-none placeholder:text-ink-dim"
        />
        <Command.List className="flex-1 overflow-y-auto p-2">
          <Command.Empty className="px-3 py-6 text-center text-sm text-ink-dim">
            No results.
          </Command.Empty>

          {matchingNav.length > 0 && (
            <Command.Group heading="Go to" className="mb-2 px-2 text-xs text-ink-dim [&_[cmdk-group-heading]]:mb-1">
              {matchingNav.map((item) => {
                const Icon = item.icon;
                return (
                  <Command.Item
                    key={item.to}
                    value={`nav-${item.to}`}
                    onSelect={() => go(`/repos/${repoId}/${item.to}`)}
                    className="flex cursor-pointer items-center gap-2.5 px-2 py-1.5 text-sm text-ink data-[selected=true]:bg-page"
                  >
                    <Icon size={14} className="text-ink-dim" />
                    {item.label}
                  </Command.Item>
                );
              })}
            </Command.Group>
          )}

          {matchingRepos.length > 0 && (
            <Command.Group heading="Switch repository" className="mb-2 px-2 text-xs text-ink-dim [&_[cmdk-group-heading]]:mb-1">
              {matchingRepos.map((r) => (
                <Command.Item
                  key={r.repo_id}
                  value={`repo-${r.repo_id}`}
                  onSelect={() => go(`/repos/${r.repo_id}`)}
                  className="cursor-pointer px-2 py-1.5 font-mono text-sm text-ink data-[selected=true]:bg-page"
                >
                  {r.repo_url.replace("https://github.com/", "")}
                </Command.Item>
              ))}
            </Command.Group>
          )}

          {debouncedQuery.length >= 2 && symbolResults.data && symbolResults.data.length > 0 && (
            <Command.Group heading="Symbols" className="mb-2 px-2 text-xs text-ink-dim [&_[cmdk-group-heading]]:mb-1">
              {symbolResults.data.slice(0, 8).map((sym, i) => (
                <Command.Item
                  key={`${sym.name}-${i}`}
                  value={`sym-${sym.name}-${i}`}
                  onSelect={() => go(`/repos/${repoId}/graph?symbol=${encodeURIComponent(sym.name)}`)}
                  className="flex cursor-pointer items-center justify-between gap-2 px-2 py-1.5 text-sm text-ink data-[selected=true]:bg-page"
                >
                  <span className="font-mono">{sym.name}</span>
                  <span className="truncate text-xs text-ink-dim">
                    {repoRelativePath(sym.file_path, repoId)}
                  </span>
                </Command.Item>
              ))}
            </Command.Group>
          )}

          {debouncedQuery.length >= 3 && searchResults.data && searchResults.data.entries && searchResults.data.entries.length > 0 && (
            <Command.Group heading="Semantic search" className="px-2 text-xs text-ink-dim [&_[cmdk-group-heading]]:mb-1">
              {searchResults.data.entries.map((entry, i) => (
                <Command.Item
                  key={`${entry.symbol}-${i}`}
                  value={`search-${entry.symbol}-${i}`}
                  onSelect={() => go(`/repos/${repoId}/graph?symbol=${encodeURIComponent(entry.symbol)}`)}
                  className="flex flex-col gap-0.5 px-2 py-1.5 text-sm text-ink data-[selected=true]:bg-page"
                >
                  <span className="font-mono">{entry.symbol}</span>
                  <span className="truncate text-xs text-ink-dim">{entry.document.slice(0, 80)}</span>
                </Command.Item>
              ))}
            </Command.Group>
          )}
        </Command.List>
      </Command>
    </Dialog>
  );
}
