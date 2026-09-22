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

// Independent phase-level outcomes (api/internal/store/job_repo.go),
// separate from the job's overall status - a job can be "completed"
// overall (parsing succeeded, the repo is genuinely usable) while
// embed_status is "failed" (e.g. the embedding provider's rate limit was
// hit). "skipped" (embed_status only) means an incremental sync found no
// changed files, so there was nothing to embed. null means that phase
// never ran at all (e.g. the job never got past cloning/walking).
export type JobPhaseStatus = "completed" | "failed" | "skipped";

export interface JobStatusResponse {
  job_id: string;
  repo_url: string;
  status: JobStatus;
  error_message: string | null;
  stage: JobStage | null;
  parse_status: JobPhaseStatus | null;
  embed_status: JobPhaseStatus | null;
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

export interface ArchitectureConcept {
  term: string;
  explanation: string;
  // Optional - present only when the concept was resolved to a real,
  // currently-indexed symbol (the LLM's own claimed location is never
  // trusted directly - see architecture.Service.GenerateOverview).
  symbol?: string;
  file_path?: string;
  start_line?: number;
  end_line?: number;
}

export interface ArchitectureReadingStep {
  symbol: string;
  file_path: string;
  reason: string;
  // 0 when no matching symbol was found in the index (e.g. an external/
  // stdlib call) - not a real line range, don't try to fetch a snippet.
  start_line: number;
  end_line: number;
}

export interface ArchitectureSummary {
  repo_id: string;
  statistics: ArchitectureStatistics;
  languages: string[];
  entrypoints: ArchitectureEntrypoint[];
  components: ArchitectureComponent[];
  narrative_summary: string;
  concepts: ArchitectureConcept[];
  reading_path: ArchitectureReadingStep[];
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
