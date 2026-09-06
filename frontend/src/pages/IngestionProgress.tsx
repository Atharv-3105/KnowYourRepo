import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { ApiError, getJobStatus, TERMINAL_JOB_STATUSES } from "../api";
import type { JobStatus } from "../api";

const STATUS_LABEL: Record<JobStatus, string> = {
  pending: "Queued",
  processing: "Reading",
  completed: "Ready",
  failed: "Failed",
};

const STATUS_CLASS: Record<JobStatus, string> = {
  pending: "border-line-faint text-line-dim",
  processing: "border-accent text-accent",
  completed: "border-line text-line",
  failed: "border-danger text-danger",
};

// job_id == repo_id in this API (see Brick 2's doc) - both POST /repos and
// POST /repos/:id/sync return a repo_id that doubles as the job id polled here.
export default function IngestionProgress() {
  const { repoId } = useParams<{ repoId: string }>();

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["job", repoId],
    queryFn: () => getJobStatus(repoId!),
    enabled: Boolean(repoId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (status && TERMINAL_JOB_STATUSES.includes(status)) return false;
      return 1500;
    },
  });

  return (
    <main className="min-h-screen">
      <div className="mx-auto max-w-xl px-6 py-20">
        <Link to="/" className="text-sm text-line-dim hover:text-line">
          &larr; repositories
        </Link>

        <h1 className="mt-4 text-2xl font-semibold text-line">Reading repository</h1>

        {isLoading && <p className="mt-4 text-sm text-line-dim">Checking status&hellip;</p>}

        {isError && (
          <p className="mt-4 text-sm text-danger">
            {error instanceof ApiError ? error.message : "Failed to check job status."}
          </p>
        )}

        {data && (
          <div className="mt-8 space-y-4">
            <div className="font-mono text-sm text-line-dim">{data.repo_url}</div>

            <span
              className={`inline-block border px-3 py-1 text-sm ${STATUS_CLASS[data.status]}`}
            >
              {STATUS_LABEL[data.status]}
            </span>

            {data.status === "failed" && data.error_message && (
              <p className="text-sm text-danger">{data.error_message}</p>
            )}

            {data.status === "completed" && (
              <div>
                <Link
                  to={`/repos/${repoId}`}
                  className="inline-block bg-accent px-5 py-2.5 text-sm font-medium text-paper-deep transition-colors hover:bg-accent-dim"
                >
                  Open repository
                </Link>
              </div>
            )}
          </div>
        )}
      </div>
    </main>
  );
}
