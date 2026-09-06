import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError, createRepo, listRepos, syncRepo } from "../api";

export default function Home() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [repoUrl, setRepoUrl] = useState("");

  const { data: repos, isLoading, isError, error } = useQuery({
    queryKey: ["repos"],
    queryFn: listRepos,
  });

  const createMutation = useMutation({
    mutationFn: createRepo,
    onSuccess: (res) => {
      navigate(`/repos/${res.repo_id}/status`);
    },
  });

  const syncMutation = useMutation({
    mutationFn: syncRepo,
    onSuccess: (res, repoId) => {
      if (res.status === "sync_queued") {
        navigate(`/repos/${repoId}/status`);
      } else {
        queryClient.invalidateQueries({ queryKey: ["repos"] });
      }
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = repoUrl.trim();
    if (trimmed) {
      createMutation.mutate(trimmed);
    }
  };

  return (
    <main className="min-h-screen">
      <div className="mx-auto max-w-3xl px-6 py-20">
        <h1 className="text-4xl font-semibold tracking-tight text-line">
          Know your repo before you touch it.
        </h1>
        <p className="mt-3 max-w-lg text-line-dim">
          Point it at a public GitHub repository. It reads the structure, indexes the code, and
          answers questions with citations back to the actual lines.
        </p>

        <form onSubmit={handleSubmit} className="mt-10 flex gap-0 border border-line-faint">
          <input
            type="url"
            required
            placeholder="https://github.com/owner/repo"
            value={repoUrl}
            onChange={(e) => setRepoUrl(e.target.value)}
            className="flex-1 bg-paper-deep px-4 py-3 font-mono text-sm text-line placeholder:text-line-dim outline-none"
          />
          <button
            type="submit"
            disabled={createMutation.isPending}
            className="border-l border-line-faint bg-accent px-6 py-3 text-sm font-medium text-paper-deep transition-colors hover:bg-accent-dim disabled:opacity-50"
          >
            {createMutation.isPending ? "Queuing" : "Read repo"}
          </button>
        </form>

        {createMutation.isError && (
          <p className="mt-2 text-sm text-danger">
            {createMutation.error instanceof ApiError
              ? createMutation.error.message
              : "Failed to queue ingestion."}
          </p>
        )}

        <div className="mt-16">
          <h2 className="text-sm text-line-dim">Indexed repositories</h2>

          {isLoading && <p className="mt-4 text-sm text-line-dim">Loading&hellip;</p>}

          {isError && (
            <p className="mt-4 text-sm text-danger">
              {error instanceof ApiError ? error.message : "Failed to load repos."}
            </p>
          )}

          {repos && repos.length === 0 && (
            <p className="mt-4 text-sm text-line-dim">None yet - read one above to get started.</p>
          )}

          {repos && repos.length > 0 && (
            <ul className="mt-4 divide-y divide-line-faint border-t border-line-faint">
              {repos.map((repo) => (
                <li key={repo.repo_id} className="flex items-center justify-between gap-4 py-3">
                  <button
                    type="button"
                    onClick={() => navigate(`/repos/${repo.repo_id}`)}
                    className="min-w-0 flex-1 text-left"
                  >
                    <div className="truncate font-mono text-sm text-line">{repo.repo_url}</div>
                    <div className="mt-0.5 text-xs text-line-dim">
                      {repo.file_count} files &middot; {repo.symbol_count} symbols &middot; indexed{" "}
                      {new Date(repo.ingested_at).toLocaleDateString()}
                    </div>
                  </button>
                  <button
                    type="button"
                    onClick={() => syncMutation.mutate(repo.repo_id)}
                    disabled={syncMutation.isPending}
                    className="shrink-0 border border-line-faint px-3 py-1.5 text-xs text-line-dim hover:border-accent hover:text-line disabled:opacity-50"
                  >
                    Sync
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </main>
  );
}
