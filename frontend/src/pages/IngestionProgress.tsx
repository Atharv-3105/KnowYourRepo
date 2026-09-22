import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { ApiError, getJobStatus, TERMINAL_JOB_STATUSES } from "../api";
import type { JobStage, JobStatus } from "../api";
import ErrorState from "../components/ErrorState";
import { SkeletonLine } from "../components/Skeleton";

const STATUS_LABEL: Record<JobStatus, string> = {
  pending: "Queued",
  processing: "Reading",
  completed: "Ready",
  failed: "Failed",
};

const STATUS_CLASS: Record<JobStatus, string> = {
  pending: "border-line-faint text-ink-dim",
  processing: "border-accent text-accent",
  completed: "border-line text-ink",
  failed: "border-danger text-danger",
};

// Real pipeline phases (Brick 12) - not a fabricated animated sequence,
// each one is set by the backend at the moment that exact phase actually
// starts. Order here matches the order the backend transitions through.
const STAGE_ORDER: JobStage[] = ["cloning", "walking", "parsing", "embedding", "done"];

const STAGE_LABEL: Record<JobStage, string> = {
  cloning: "Cloning repository",
  walking: "Walking file tree",
  parsing: "Parsing symbols",
  embedding: "Generating embeddings",
  done: "Finishing up",
};

// job_id == repo_id in this API (see Brick 2's doc) - both POST /repos and
// POST /repos/:id/sync return a repo_id that doubles as the job id polled here.
export default function IngestionProgress() {
  const { repoId } = useParams<{ repoId: string }>();

  const { data, isLoading, isError, error, refetch } = useQuery({
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
        <Link to="/" className="text-sm text-ink-dim hover:text-ink">
          &larr; repositories
        </Link>

        <h1 className="mt-4 font-display text-3xl font-medium text-ink">Reading repository</h1>

        {isLoading && (
          <div className="mt-8 space-y-3">
            <SkeletonLine className="w-56" />
            <SkeletonLine className="w-24" />
          </div>
        )}

        {isError && (
          <div className="mt-8">
            <ErrorState
              message={error instanceof ApiError ? error.message : "Failed to check job status."}
              onRetry={() => refetch()}
            />
          </div>
        )}

        {data && (
          <div className="mt-8 space-y-4">
            <div className="font-mono text-sm text-ink-dim">{data.repo_url}</div>

            <span
              className={`inline-block border px-3 py-1 text-sm ${STATUS_CLASS[data.status]}`}
            >
              {STATUS_LABEL[data.status]}
            </span>

            {data.status === "processing" && data.stage && (
              <ul className="space-y-1.5 font-mono text-xs">
                {STAGE_ORDER.map((stage) => {
                  const currentIndex = STAGE_ORDER.indexOf(data.stage!);
                  const stageIndex = STAGE_ORDER.indexOf(stage);
                  const reached = stageIndex < currentIndex;
                  const active = stageIndex === currentIndex;
                  return (
                    <li
                      key={stage}
                      className={reached ? "text-ink-dim" : active ? "text-accent" : "text-ink-faint"}
                    >
                      {reached ? "✓" : active ? "›" : "·"} {STAGE_LABEL[stage]}
                    </li>
                  );
                })}
              </ul>
            )}

            {data.status === "failed" && data.error_message && (
              <p className="text-sm text-danger">{data.error_message}</p>
            )}

            {/* A completed job can still have a failed embed_status (e.g.
                the embedding provider's rate limit was hit) - the repo is
                genuinely usable (browsable, has a call graph, Chat still
                works via lexical/graph-fallback retrieval), just without
                semantic search until the next sync retries embedding. Said
                plainly rather than hidden behind a flat "Ready" badge. */}
            {data.status === "completed" && data.embed_status === "failed" && (
              <p className="text-sm text-ink-dim">
                Repository is ready, but semantic search isn&apos;t available yet - embedding
                failed (the provider may be rate-limited). It will retry on the next sync.
              </p>
            )}

            {data.status === "completed" && (
              <div>
                <Link
                  to={`/repos/${repoId}`}
                  className="inline-block bg-accent px-5 py-2.5 text-sm font-medium text-page-deep transition-colors hover:bg-accent-dim"
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
