import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { listRepos } from "../api";

interface RepoSwitcherProps {
  currentRepoId?: string;
}

// A plain <select>, not a custom dropdown - matches the pattern already
// used for GraphView's root-symbol picker, and a repo switch is a rare,
// deliberate action, not something that needs a richer widget.
export default function RepoSwitcher({ currentRepoId }: RepoSwitcherProps) {
  const navigate = useNavigate();

  const { data: repos } = useQuery({
    queryKey: ["repos"],
    queryFn: listRepos,
  });

  if (!repos || repos.length === 0) return null;

  return (
    <select
      value={currentRepoId ?? ""}
      onChange={(e) => navigate(`/repos/${e.target.value}`)}
      className="max-w-[220px] truncate border border-line-faint bg-page px-2 py-1 font-mono text-xs text-ink-dim hover:text-ink"
    >
      {!currentRepoId && <option value="" disabled>select a repo</option>}
      {repos.map((repo) => (
        <option key={repo.repo_id} value={repo.repo_id}>
          {repo.repo_url.replace("https://github.com/", "")}
        </option>
      ))}
    </select>
  );
}
