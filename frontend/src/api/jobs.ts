import { api } from "./client";
import type { JobStatusResponse } from "./types";

// job_id == repo_id in this API - both POST /repos and POST /repos/:id/sync
// return a repo_id that doubles as the job id to poll here.
export const getJobStatus = (jobId: string) =>
  api.get<JobStatusResponse>(`/repos/jobs/${jobId}`);
