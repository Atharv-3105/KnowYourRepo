// These mirror the actual Go response structs verified live against the
// running API during Bricks 1-4 (see Docs/phase5_frontend) - not guessed
// from documentation.

export interface RepoSummary {
  repo_id: string;
  repo_url: string;
  ingested_at: string;
  file_count: number;
  symbol_count: number;
}

export interface CreateRepoResponse {
  success: boolean;
  message: string;
  repo_id: string;
}

export type SyncRepoResponse =
  | { status: "up_to_date" }
  | { status: "sync_queued"; repo_id: string };

export type JobStatus = "pending" | "processing" | "completed" | "failed";

// Real, sequential pipeline phases (api/internal/store/job_repo.go's
// Stage* constants, Brick 12) - null until the worker actually picks the
// job up, since a queued-but-not-yet-started job has no stage yet.
export type JobStage = "cloning" | "walking" | "parsing" | "embedding" | "done";

export interface JobStatusResponse {
  job_id: string;
  repo_url: string;
  status: JobStatus;
  error_message: string | null;
  stage: JobStage | null;
}

// A job is done polling once it reaches either terminal state.
export const TERMINAL_JOB_STATUSES: JobStatus[] = ["completed", "failed"];

export interface Source {
  symbol: string;
  file_path: string;
  start_line?: number;
  end_line?: number;
}

export interface AgentChatRequest {
  repo_id: string;
  question: string;
  session_id: string;
}

export interface AgentChatResponse {
  answer: string;
  tools_used: string[];
  refreshing: boolean;
  sources: Source[];
}

export interface ArchitectureStatistics {
  file_count: number;
  symbol_count: number;
  call_edges: number;
}

export interface ArchitectureEntrypoint {
  name: string;
  file_path: string;
  language: string;
  type: string;
}

export interface ArchitectureComponent {
  name: string;
  file_path: string;
  type: string;
  language: string;
  start_line: number;
  end_line: number;
}

export interface ArchitectureSummary {
  repo_id: string;
  statistics: ArchitectureStatistics;
  languages: string[];
  entrypoints: ArchitectureEntrypoint[];
  components: ArchitectureComponent[];
}

export interface GraphNode {
  symbol: string;
  // Omitted by the backend (omitempty) for symbols never seen as a caller
  // in the traversal - e.g. stdlib/external calls. See Brick 3's doc.
  file_path?: string;
}

export interface GraphEdge {
  caller: string;
  callee: string;
}

export interface BoundedGraphResponse {
  repo_id: string;
  root_symbol: string;
  depth: number;
  truncated: boolean;
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface FileContentResponse {
  repo_id: string;
  path: string;
  content: string;
}

export interface SymbolEntry {
  name: string;
  type: string;
  language: string;
  file_path: string;
  start_line: number;
  end_line: number;
}

// POST /search's response (contextbuilder.ContextPackage) - the same
// semantic-retrieval shape used internally to build the agent's LLM
// prompt, exposed directly here. No file/line data (see Brick 4's doc on
// why citations moved onto AgentChatResponse.sources instead of this
// endpoint).
export interface SearchEntry {
  symbol: string;
  document: string;
  calls: string[] | null;
}

export interface SearchResponse {
  query: string;
  entries: SearchEntry[] | null;
}
