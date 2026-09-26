package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/config"

	"github.com/atharva-3105/KnowYourRepo/internal/answer"
	"github.com/atharva-3105/KnowYourRepo/internal/architecture"
	"github.com/atharva-3105/KnowYourRepo/internal/chat"
	"github.com/atharva-3105/KnowYourRepo/internal/chunk"
	"github.com/atharva-3105/KnowYourRepo/internal/contextbuilder"
	"github.com/atharva-3105/KnowYourRepo/internal/graph"
	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
	"github.com/atharva-3105/KnowYourRepo/internal/metrics"
	"github.com/atharva-3105/KnowYourRepo/internal/rag"
	"github.com/atharva-3105/KnowYourRepo/internal/representation"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
	"github.com/atharva-3105/KnowYourRepo/internal/worker"
	"github.com/gin-gonic/gin"
)

const (
	IngestionQueueSize = 16

	// staleCheckCooldown bounds how often a cached-repo request triggers a
	// background "has the remote moved?" check - each check is a real
	// network round trip to GitHub.
	staleCheckCooldown = 10 * time.Minute
)

type RepoHandler struct {
	logger              *slog.Logger
	store               *store.Store
	sidecar             *sidecar.Client
	parser              *ingestion.TreeSitterParser
	extractor           *graph.Extractor
	cloner              *ingestion.Cloner
	walker              *ingestion.Walker
	github              *ingestion.GitHubClient
	hybridRetriever     *retrieval.HybridRetriever
	contextBuilder      *contextbuilder.Builder
	ragService          *rag.Service
	chatStore           *chat.Store
	architectureService *architecture.Service
	answerService       *answer.Service
	workerPool          *worker.Pool
	limits              config.LimitsConfig

	shuttingDown atomic.Bool
	stopJanitor  chan struct{}
}

func NewRepoHandler(
	cfg *config.Config,
	logger *slog.Logger,
	store *store.Store,
	sidecar *sidecar.Client,
) *RepoHandler {

	builder := contextbuilder.NewBuilder(logger)
	architectureAnalyzer := architecture.NewAnalyzer(logger, store)
	architectureService := architecture.NewService(logger, architectureAnalyzer, store, sidecar)

	hybridRetriever := retrieval.NewHybridRetriever(store, sidecar, logger)
	ragService := rag.NewService(builder, sidecar, logger)

	h := &RepoHandler{
		logger:              logger,
		store:               store,
		sidecar:             sidecar,
		parser:              ingestion.NewParser(logger),
		extractor:           graph.NewExtractor(logger),
		cloner:              ingestion.NewCloner(logger),
		walker:              ingestion.NewWalker(logger).WithLimits(cfg.Limits.MaxRepoFiles, cfg.Limits.MaxFileBytes),
		github:              ingestion.NewGitHubClient(cfg.GitHub.Token),
		hybridRetriever:     hybridRetriever,
		contextBuilder:      builder,
		ragService:          ragService,
		chatStore:           chat.NewStore(cfg.Limits.MaxChatSessions, cfg.Limits.ChatSessionTTL),
		architectureService: architectureService,
		limits:              cfg.Limits,
		stopJanitor:         make(chan struct{}),
	}

	h.chatStore.StartJanitor(10*time.Minute, h.stopJanitor)

	// answerService needs h as its RepoSyncer (for freshness-triggered
	// background sync), so it's constructed after h exists - same
	// deferred-wiring pattern as workerPool below.
	h.answerService = answer.NewService(hybridRetriever, architectureService, ragService, h, logger)

	h.workerPool = worker.NewPool(cfg.Limits.IngestionWorkers, IngestionQueueSize, h.handleIngestionJob, logger)
	h.workerPool.Start(context.Background())

	return h
}

// ResumeUnfinishedJobs re-enqueues jobs left pending or processing by a
// previous process - a deploy, a crash, or a free-tier instance spinning
// down mid-ingestion. Ingestion is incremental (unchanged files are skipped
// by hash), so resuming a half-finished job redoes only the unfinished part.
func (h *RepoHandler) ResumeUnfinishedJobs(ctx context.Context) {
	ids, err := h.store.ListUnfinishedJobIDs(ctx)
	if err != nil {
		h.logger.Error("resume_unfinished_jobs_failed", "error", err)
		return
	}

	for _, id := range ids {
		if !h.workerPool.Enqueue(id) {
			msg := "ingestion queue was full after a server restart - please resubmit"
			if err := h.store.UpdateJobStatus(ctx, id, store.JobStatusFailed, &msg); err != nil {
				h.logger.Error("failed to mark unresumable job failed", "job_id", id, "error", err)
			}
			continue
		}
		h.logger.Info("ingestion_job_resumed", "job_id", id)
	}
}

// Shutdown stops accepting ingestion work and waits (up to ctx's deadline)
// for in-flight jobs; see worker.Pool.Shutdown.
func (h *RepoHandler) Shutdown(ctx context.Context) error {
	h.shuttingDown.Store(true)
	close(h.stopJanitor)
	return h.workerPool.Shutdown(ctx)
}

// CreateRepo handles POST /repos.
//
// Three outcomes, cheapest first:
//   - The URL is already being ingested: return that job (202). Enqueueing
//     it again would put two workers on the same clone directory.
//   - The URL was already ingested successfully: return it immediately
//     (200, cached) - the index, embeddings and overview already exist, so
//     re-running the pipeline would spend provider quota to reproduce them.
//     A background staleness check still catches it up if the remote moved.
//   - Otherwise (new URL, or a previous attempt failed): queue ingestion (202).
func (h *RepoHandler) CreateRepo(c *gin.Context) {

	var req CreateRepoRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	repoURL, err := NormalizeGitHubURL(req.RepoURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	active, err := h.store.GetActiveJobByURL(ctx, repoURL)
	if err != nil {
		respondInternalError(c, h.logger, "create_repo_active_job_lookup_failed", err)
		return
	}
	if active != nil {
		h.logger.Info("ingestion_already_in_progress", "repo_id", active.ID, "repo_url", repoURL)
		c.JSON(http.StatusAccepted, CreateRepoResponse{
			Success: true,
			Message: "ingestion already in progress",
			RepoID:  active.ID,
		})
		return
	}

	existingRepo, err := h.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		respondInternalError(c, h.logger, "create_repo_lookup_failed", err)
		return
	}

	var repoID string

	if existingRepo != nil {
		repoID = existingRepo.ID

		job, err := h.store.FindJob(ctx, repoID)
		if err != nil {
			respondInternalError(c, h.logger, "create_repo_job_lookup_failed", err, "repo_id", repoID)
			return
		}

		if job != nil && job.Status == store.JobStatusCompleted {
			h.logger.Info("repo_cache_hit", "repo_id", repoID, "repo_url", repoURL)
			metrics.RepoCacheHitsTotal.Inc()
			h.checkStalenessInBackground(repoID, existingRepo.RepoURL)

			c.JSON(http.StatusOK, CreateRepoResponse{
				Success: true,
				Message: "repository already ingested",
				RepoID:  repoID,
				Cached:  true,
			})
			return
		}

		h.logger.Info("repo_requeued_after_unfinished_attempt", "repo_id", repoID, "repo_url", repoURL)
	} else {
		// The repositories row itself is NOT inserted here - it's inserted
		// by ingestRepository only after the clone actually succeeds, so a
		// failed clone never leaves a "ghost" repository.
		repoID = generateRepoDirName()
	}

	if err := h.store.InsertJob(ctx, repoID, repoURL); err != nil {
		respondInternalError(c, h.logger, "create_repo_insert_job_failed", err, "repo_id", repoID)
		return
	}

	if !h.workerPool.Enqueue(repoID) {

		errMsg := "ingestion queue is full, try again shortly"

		if updateErr := h.store.UpdateJobStatus(ctx, repoID, store.JobStatusFailed, &errMsg); updateErr != nil {
			h.logger.Error("failed to mark job failed", "job_id", repoID, "error", updateErr)
		}

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": errMsg})
		return
	}

	h.logger.Info("ingestion_job_queued", "repo_id", repoID, "repo_url", repoURL)

	c.JSON(http.StatusAccepted, CreateRepoResponse{
		Success: true,
		Message: "ingestion queued",
		RepoID:  repoID,
	})
}

// checkStalenessInBackground runs the cheap remote-HEAD comparison for a
// cached repo (no clone, no disk write) and enqueues a sync only if the
// remote actually moved - rate-limited by staleCheckCooldown so repeated
// submissions of a popular URL don't each cost a GitHub round trip.
func (h *RepoHandler) checkStalenessInBackground(repoID, repoURL string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		lastChecked, ok, err := h.store.GetLastCheckedAt(ctx, repoID)
		if err != nil {
			h.logger.Warn("staleness_check_lookup_failed", "repo_id", repoID, "error", err)
			return
		}
		if ok && time.Since(lastChecked) < staleCheckCooldown {
			return
		}

		if _, err := h.checkAndEnqueueSync(ctx, repoID, repoURL); err != nil {
			h.logger.Warn("staleness_check_failed", "repo_id", repoID, "error", err)
		}
	}()
}

// requireRepoExists checks that repoID names a repository that actually
// completed ingestion. Every read endpoint scoped by repoID (architecture,
// symbols, bounded graph, ...) queries tables that are simply empty for any
// unknown ID - without this check they return a misleading 200 with
// zeroed-out data instead of a 404, indistinguishable from a real repo that
// just happens to have no files yet. Returns true and writes nothing if the
// repo exists; otherwise writes the appropriate error response itself and
// returns false, so callers can just `if !h.requireRepoExists(c, repoID) { return }`.
func (h *RepoHandler) requireRepoExists(c *gin.Context, repoID string) bool {

	repo, err := h.store.GetRepositoryByID(c.Request.Context(), repoID)
	if err != nil {
		respondInternalError(c, h.logger, "repo_lookup_failed", err, "repo_id", repoID)
		return false
	}

	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return false
	}

	return true
}

// handleIngestionJob adapts ingestRepository to the worker.Handler
// signature, recording the job's terminal status once ingestion finishes
// (successfully or not). Uses a fresh background context for the status
// write so a cancelled/timed-out ingestion ctx doesn't also prevent
// recording that it failed.
func (h *RepoHandler) handleIngestionJob(ctx context.Context, jobID string) error {

	start := time.Now()

	err := h.ingestRepository(ctx, jobID)

	statusCtx := context.Background()

	if err != nil {
		// Interrupted by shutdown (a deploy, or a free-tier instance
		// spinning down): leave the job pending so ResumeUnfinishedJobs
		// picks it up on the next start, instead of reporting a failure
		// the user didn't cause.
		if h.shuttingDown.Load() && ctx.Err() != nil {
			msg := "interrupted by a server restart - it will resume automatically"
			if updateErr := h.store.UpdateJobStatus(statusCtx, jobID, store.JobStatusPending, &msg); updateErr != nil {
				h.logger.Error("failed to mark interrupted job pending", "job_id", jobID, "error", updateErr)
			}
			h.logger.Warn("ingestion_interrupted_by_shutdown", "job_id", jobID)
			return err
		}

		metrics.IngestionJobDuration.WithLabelValues("failed").Observe(time.Since(start).Seconds())

		msg := userFacingJobError(err)
		if updateErr := h.store.UpdateJobStatus(statusCtx, jobID, store.JobStatusFailed, &msg); updateErr != nil {
			h.logger.Error("failed to mark job failed", "job_id", jobID, "error", updateErr)
		}
		return err
	}

	metrics.IngestionJobDuration.WithLabelValues("completed").Observe(time.Since(start).Seconds())

	if updateErr := h.store.UpdateJobStatus(statusCtx, jobID, store.JobStatusCompleted, nil); updateErr != nil {
		h.logger.Error("failed to mark job completed", "job_id", jobID, "error", updateErr)
	}

	return nil
}

// ingestRepository runs the clone-or-sync -> walk -> diff -> parse ->
// extract -> store -> chunk -> embed pipeline. jobID and repoID are the
// same value (see job_repo.go) - the job row is the source of truth for
// repoURL so the worker only needs to pass a single ID around.
//
// Incremental behaviour: files whose content hash matches what's already
// stored are skipped entirely (no parse, no re-embed). Changed files have
// their stale symbols/call_edges/embeddings purged before being
// re-processed. Files no longer present on disk are removed the same way.
func (h *RepoHandler) ingestRepository(ctx context.Context, jobID string) error {

	job, err := h.store.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to load job: %w", err)
	}

	if err := h.store.UpdateJobStatus(ctx, jobID, store.JobStatusProcessing, nil); err != nil {
		h.logger.Error("failed to mark job processing", "job_id", jobID, "error", err)
	}

	if err := h.store.UpdateJobStage(ctx, jobID, store.StageCloning); err != nil {
		h.logger.Error("failed to mark job stage cloning", "job_id", jobID, "error", err)
	}

	repoID := jobID
	repoURL := job.RepoURL

	repoDir := repoDirFor(repoID)

	h.logger.Info("starting repo ingestion", "repo_url", repoURL)

	// Fail fast on a repository that's too large for this deployment (or
	// doesn't exist) before spending disk, memory and time cloning it.
	if err := h.github.CheckRepoSize(ctx, repoURL, h.limits.MaxRepoSizeMB); err != nil {
		return err
	}

	//Clone if this is the first time, sync (fetch + hard reset) if we've
	//already cloned this repo before.
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {

		// A directory without .git is a clone that was interrupted partway
		// (e.g. by a restart) - CloneRepo refuses a non-empty destination,
		// so clear it and start over.
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to clear partial clone: %w", err)
		}

		if err := h.cloner.CloneRepo(ctx, repoURL, repoDir); err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}

		//Only now that the clone actually succeeded does this repo become a
		//real, browsable entity - see the comment in CreateRepo for why this
		//moved out of the synchronous POST /repos path.
		// The row can already exist when an existing repo is re-cloned
		// (e.g. its clone directory was lost on an ephemeral free-tier
		// disk) - only insert if it's genuinely new.
		existing, err := h.store.GetRepositoryByID(ctx, repoID)
		if err != nil {
			return fmt.Errorf("failed to check repository record: %w", err)
		}
		if existing == nil {
			if err := h.store.InsertRepository(ctx, store.Repository{
				ID:      repoID,
				RepoURL: repoURL,
			}); err != nil {
				return fmt.Errorf("failed to record repository: %w", err)
			}
		}

	} else {

		if err := h.cloner.SyncRepo(ctx, repoURL, repoDir); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}
	}

	if commitSHA, err := h.cloner.HeadCommitSHA(repoDir); err != nil {
		h.logger.Warn("failed to resolve head commit sha", "error", err)
	} else if err := h.store.TouchRepoCheck(ctx, repoID, commitSHA); err != nil {
		h.logger.Error("failed to record repo sync state", "error", err)
	}

	// The current embedding model decides which already-indexed files still
	// need (re-)embedding. If the sidecar can't say, unchanged files are
	// skipped as usual and nothing is marked embedded.
	embedModel := ""
	if info, err := h.sidecar.EmbedInfo(ctx); err != nil {
		h.logger.Warn("embed_info_unavailable", "repo_id", repoID, "error", err)
	} else {
		embedModel = info.Model
	}

	if err := h.store.UpdateJobStage(ctx, jobID, store.StageWalking); err != nil {
		h.logger.Error("failed to mark job stage walking", "job_id", jobID, "error", err)
	}

	//Get the files of the Repository
	files, err := h.walker.WalkRepo(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	h.logger.Info("found files", "count", len(files))

	//Load existing file hashes for incremental diffing
	existing, err := h.store.GetFileHashesByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to load existing file hashes: %w", err)
	}

	repoIR := representation.Repository{
		Name: filepath.Base(repoDir),
	}

	var embedItems []sidecar.EmbedBatchItem
	var spans []fileChunkSpan
	seenPaths := make(map[string]bool)

	// processed = new or changed content; reembedded = unchanged content
	// whose embedding is missing or from another model.
	var skipped, processed, reembedded int

	if err := h.store.UpdateJobStage(ctx, jobID, store.StageParsing); err != nil {
		h.logger.Error("failed to mark job stage parsing", "job_id", jobID, "error", err)
	}

	//Process the files
	for _, file := range files {

		seenPaths[file.Path] = true

		hash, err := ingestion.HashFile(file.Path)
		if err != nil {
			h.logger.Warn("failed to hash file", "path", file.Path, "error", err)
			continue
		}

		prior, hadPrior := existing[file.Path]
		contentUnchanged := hadPrior && prior.Hash == hash
		needsEmbedding := embedModel != "" && prior.EmbeddedModel != embedModel

		if contentUnchanged && !needsEmbedding {
			//Unchanged and embedded with the current model - skip parsing/extraction/embedding entirely
			skipped++
			continue
		}

		if hadPrior {
			//Existing file whose content changed - clear stale symbols/call_edges/embeddings before re-processing
			if err := h.store.DeleteSymbolsAndCallEdges(ctx, repoID, prior.ID, file.Path); err != nil {
				h.logger.Error("failed to clear stale symbols/call_edges", "path", file.Path, "error", err)
			}

			if err := h.sidecar.DeleteEmbeddings(ctx, sidecar.DeleteEmbeddingsRequest{RepoID: repoID, FilePath: file.Path}); err != nil {
				h.logger.Warn("failed to delete stale embeddings", "path", file.Path, "error", err)
			}
		}

		items, irFile, err := h.processFile(ctx, repoID, file, hash)
		if err != nil {
			h.logger.Warn("failed to process file", "path", file.Path, "error", err)
			continue
		}

		spans = append(spans, fileChunkSpan{path: file.Path, start: len(embedItems), end: len(embedItems) + len(items)})
		embedItems = append(embedItems, items...)
		repoIR.Files = append(repoIR.Files, irFile)
		if contentUnchanged {
			reembedded++
		} else {
			processed++
		}
	}

	//Anything previously known but not seen on this walk was deleted upstream
	for path := range existing {

		if seenPaths[path] {
			continue
		}

		if err := h.store.DeleteFileByPath(ctx, repoID, path); err != nil {
			h.logger.Error("failed to delete stale file record", "path", path, "error", err)
		}

		if err := h.sidecar.DeleteEmbeddings(ctx, sidecar.DeleteEmbeddingsRequest{RepoID: repoID, FilePath: path}); err != nil {
			h.logger.Warn("failed to delete stale embeddings", "path", path, "error", err)
		}
	}

	h.logger.Info("incremental_diff_complete", "processed", processed, "reembedded", reembedded, "skipped", skipped, "deleted", len(existing)-len(seenPaths))

	// Parsing/symbol/call-graph extraction just finished, unconditionally -
	// if we've reached this point, the repo already has real, usable
	// structural data (browsable, has a call graph, Chat still works via
	// lexical/graph-fallback retrieval) regardless of what embedding does
	// next. Tracked independently of the job's overall status - see the
	// schema.sql comment on parse_status/embed_status for why.
	if err := h.store.SetParseStatus(ctx, jobID, store.JobStatusCompleted); err != nil {
		h.logger.Error("failed to set job parse_status", "job_id", jobID, "error", err)
	}

	if err := h.store.UpdateJobStage(ctx, jobID, store.StageEmbedding); err != nil {
		h.logger.Error("failed to mark job stage embedding", "job_id", jobID, "error", err)
	}

	// Free-tier guardrail: embedding is the one pipeline stage that spends
	// provider quota per chunk. Past the cap, the remaining chunks are still
	// parsed, stored and reachable through lexical and call-graph retrieval -
	// they just aren't semantically searchable.
	if h.limits.MaxEmbedChunks > 0 && len(embedItems) > h.limits.MaxEmbedChunks {
		h.logger.Warn("embed_chunk_cap_applied", "repo_id", repoID, "chunks", len(embedItems), "cap", h.limits.MaxEmbedChunks)
		metrics.EmbedChunksTotal.WithLabelValues("capped").Add(float64(len(embedItems) - h.limits.MaxEmbedChunks))
		embedItems = embedItems[:h.limits.MaxEmbedChunks]
	}

	embedded := 0
	if len(embedItems) > 0 {

		h.logger.Info("sending batch embeddings", "count", len(embedItems))

		// Non-fatal, deliberately: previously this returned early on any
		// embedding failure (e.g. the embedding provider's rate limit),
		// which meant the whole job was reported "failed" even though
		// parsing above already succeeded and the repo's symbols/call-graph
		// were already committed - a real, usable repo reported as if
		// ingestion never happened. It also meant GenerateOverview below,
		// which doesn't actually depend on embeddings, silently never ran.
		// embed_status carries the real, separate outcome instead.
		var embedErr error
		embedded, embedErr = h.embedInBatches(ctx, repoID, embedItems)
		if err := embedErr; err != nil {
			h.logger.Warn("batch_embedding_failed", "repo_id", repoID, "embedded", embedded, "error", err)
			if statusErr := h.store.SetEmbedStatus(ctx, jobID, store.JobStatusFailed); statusErr != nil {
				h.logger.Error("failed to set job embed_status", "job_id", jobID, "error", statusErr)
			}
		} else if err := h.store.SetEmbedStatus(ctx, jobID, store.JobStatusCompleted); err != nil {
			h.logger.Error("failed to set job embed_status", "job_id", jobID, "error", err)
		}
	} else if err := h.store.SetEmbedStatus(ctx, jobID, store.JobStatusSkipped); err != nil {
		h.logger.Error("failed to set job embed_status", "job_id", jobID, "error", err)
	}

	// Files whose chunks all made it in are done; the rest (quota ran out,
	// or past the per-run chunk budget) are picked up by the next sync.
	if embedModel != "" {
		if err := h.store.MarkFilesEmbedded(ctx, repoID, fullyEmbeddedFiles(spans, embedded), embedModel); err != nil {
			h.logger.Error("failed to mark files embedded", "repo_id", repoID, "error", err)
		}
	}

	//Save IR File Representation
	irPath := filepath.Join(repoDir, "repository_ir.json")

	if err := representation.SaveRepository(repoIR, irPath); err != nil {
		h.logger.Error("failed to save repository IR", "error", err)
	}

	readmeText, err := h.walker.ReadReadme(repoDir)
	if err != nil {
		h.logger.Warn("failed to read readme", "repo_id", repoID, "error", err)
		readmeText = ""
	}

	// Non-fatal: overview generation failing (all LLM providers exhausted,
	// zero entrypoints detected, etc.) must never fail the whole ingestion
	// job - symbols/embeddings are the core value, the overview is
	// supplementary. See the design spec's Error handling section.
	deleted := len(existing) - len(seenPaths)
	if deleted < 0 {
		deleted = 0
	}
	if h.shouldGenerateOverview(ctx, repoID, processed, deleted) {
		if err := h.architectureService.GenerateOverview(ctx, repoID, readmeText); err != nil {
			h.logger.Warn("overview_generation_failed", "repo_id", repoID, "error", err)
		}
	} else {
		h.logger.Info("overview_generation_skipped_no_changes", "repo_id", repoID)
	}

	if err := h.store.UpdateJobStage(ctx, jobID, store.StageDone); err != nil {
		h.logger.Error("failed to mark job stage done", "job_id", jobID, "error", err)
	}

	h.logger.Info("repo ingestion complete", "repo_id", repoID)

	return nil
}

// processFile parses, extracts symbols/call-graph, and prepares embed
// items for a single new-or-changed file. It's the per-file body that
// ingestRepository's old inline loop used to do directly - pulled out so
// the skip/changed/new branching logic above stays readable.
func (h *RepoHandler) processFile(ctx context.Context, repoID string, file ingestion.FileInfo, hash string) ([]sidecar.EmbedBatchItem, representation.File, error) {

	fileID, err := h.store.InsertFile(ctx, repoID, file.Path, file.Language, hash)
	if err != nil {
		return nil, representation.File{}, fmt.Errorf("failed to insert file: %w", err)
	}

	parseResult, err := h.parser.ParseFile(ctx, file.Path, file.Language)
	if err != nil {
		return nil, representation.File{}, fmt.Errorf("parsing failed: %w", err)
	}

	chunks := chunk.ExtractFunctionChunks(parseResult.Root, parseResult.Source, file.Language, file.Path)

	var finalChunks []chunk.Chunk
	for _, ch := range chunks {
		finalChunks = append(finalChunks, chunk.SplitChunk(ch)...)
	}

	irFile := representation.NewFile(file.Path, file.Language)

	symbols, err := h.extractor.ExtractSymbols(parseResult.Root, parseResult.Source, file.Language)
	if err != nil {
		return nil, representation.File{}, fmt.Errorf("symbol extraction failed: %w", err)
	}

	var edges []graph.CallEdge

	switch file.Language {
	case "go":
		edges = graph.ExtractGoCallGraph(parseResult.Root, parseResult.Source)
	case "python":
		edges = graph.ExtractPythonCallGraph(parseResult.Root, parseResult.Source)
	case "javascript", "typescript":
		edges = graph.ExtractJSCallGraph(parseResult.Root, parseResult.Source)
	}

	for _, edge := range edges {

		if err := h.store.InsertCallEdge(ctx, store.CallEdge{
			RepoID:         repoID,
			CallerSymbol:   edge.Caller,
			CallerFilePath: file.Path,
			CalleeSymbol:   edge.Callee,
		}); err != nil {
			h.logger.Error("failed to store call edge", "caller", edge.Caller, "callee", edge.Callee, "error", err)
		}

		irFile.Calls = append(irFile.Calls, representation.CallEdge{Caller: edge.Caller, Callee: edge.Callee})
	}

	for _, sym := range symbols {

		symbolID, err := h.store.InsertSymbol(ctx, store.Symbol{
			FileID:    fileID,
			Name:      sym.Name,
			Type:      sym.Type,
			StartLine: sym.StartLine,
			EndLine:   sym.EndLine,
		})

		if err != nil {
			h.logger.Error("failed to insert symbol", "error", err)
			continue
		}

		irFile.Symbols = append(irFile.Symbols, representation.Symbol{
			ID:        fmt.Sprintf("%d", symbolID),
			Name:      sym.Name,
			Kind:      sym.Type,
			Language:  file.Language,
			FilePath:  file.Path,
			StartLine: sym.StartLine,
			EndLine:   sym.EndLine,
		})
	}

	var items []sidecar.EmbedBatchItem

	for idx, ch := range finalChunks {

		items = append(items, sidecar.EmbedBatchItem{
			ID:   fmt.Sprintf("%s_part_%d", ch.ID, idx),
			Text: ch.Content,
			Metadata: map[string]interface{}{
				"repo_id":    repoID,
				"file_path":  ch.FilePath,
				"language":   ch.Language,
				"symbol":     ch.SymbolName,
				"start_line": ch.StartLine,
				"end_line":   ch.EndLine,
				"chunk_size": len(ch.Content),
			},
		})
	}

	return items, irFile, nil
}

// checkAndEnqueueSync does the cheap ls-remote-equivalent staleness check
// (no clone, no disk write) and enqueues a real sync only if the remote
// HEAD actually moved. Shared by the on-demand /repos/:id/sync endpoint
func (h *RepoHandler) checkAndEnqueueSync(ctx context.Context, repoID, repoURL string) (enqueued bool, err error) {

	// Already queued or running - a second job would race the first on the
	// same clone directory.
	if active, err := h.store.GetActiveJobByURL(ctx, repoURL); err != nil {
		return false, fmt.Errorf("failed to check for an active job: %w", err)
	} else if active != nil {
		return true, nil
	}

	// On hosts with an ephemeral filesystem (most free tiers), the clone
	// directory is gone after every restart even though the index in
	// Postgres is intact. The remote SHA would still match, so without this
	// the repo would read as "up to date" forever with no files on disk for
	// the code view. Re-syncing re-clones, and every file is then skipped by
	// hash - no re-parsing, no re-embedding.
	cloneMissing := !h.cloneExists(repoID)

	// Files left unembedded by an exhausted daily quota, the per-run chunk
	// budget, or an embedding-model change get another pass.
	pendingEmbedding := h.hasPendingEmbedding(ctx, repoID)

	remoteSHA, err := ingestion.RemoteHeadSHA(ctx, repoURL)
	if err != nil {
		return false, fmt.Errorf("failed to check remote head: %w", err)
	}

	localSHA, err := h.store.GetLastCommitSHA(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("failed to get last known commit sha: %w", err)
	}

	if remoteSHA == localSHA && !cloneMissing && !pendingEmbedding {
		if err := h.store.TouchRepoCheck(ctx, repoID, localSHA); err != nil {
			h.logger.Error("failed to record repo check", "repo_id", repoID, "error", err)
		}
		return false, nil
	}

	if err := h.store.InsertJob(ctx, repoID, repoURL); err != nil {
		return false, fmt.Errorf("failed to queue sync job: %w", err)
	}

	if !h.workerPool.Enqueue(repoID) {
		return false, fmt.Errorf("ingestion queue is full")
	}

	h.logger.Info("repo_sync_enqueued", "repo_id", repoID, "remote_sha", remoteSHA, "local_sha", localSHA, "clone_missing", cloneMissing, "pending_embedding", pendingEmbedding)

	return true, nil
}

// SyncRepo handles POST /repos/:id/sync - an on-demand "check for updates
// now" trigger (e.g. a frontend refresh button), separate from the
// periodic background scheduler
func (h *RepoHandler) SyncRepo(c *gin.Context) {

	repoID := c.Param("id")

	ctx := c.Request.Context()

	repo, err := h.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		respondInternalError(c, h.logger, "sync_repo_lookup_failed", err, "repo_id", repoID)
		return
	}

	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	enqueued, err := h.checkAndEnqueueSync(ctx, repo.ID, repo.RepoURL)
	if err != nil {
		respondInternalError(c, h.logger, "sync_repo_failed", err, "repo_id", repoID)
		return
	}

	if !enqueued {
		c.JSON(http.StatusOK, gin.H{"status": "up_to_date"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "sync_queued", "repo_id": repo.ID})
}

// SyncIfStale implements answer.RepoSyncer - looks up the repo's URL and
// runs the same cheap staleness check + background enqueue used by the
// on-demand /repos/:id/sync endpoint. Called by the answer service when a
// chat question implies the user wants the repo's latest state.
func (h *RepoHandler) SyncIfStale(ctx context.Context, repoID string) error {

	repo, err := h.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}

	if repo == nil {
		return fmt.Errorf("repository not found: %s", repoID)
	}

	_, err = h.checkAndEnqueueSync(ctx, repo.ID, repo.RepoURL)
	return err
}

// repoDirFor is where a repository's working copy lives, relative to the
// API's working directory (api/) - see CLAUDE.md on why it's relative.
func repoDirFor(repoID string) string {
	return filepath.Join("..", "data", "repos", repoID)
}

// hasPendingEmbedding reports whether a sync would embed anything: some
// files aren't embedded with the current model, and the quota isn't known
// to be exhausted (a sync then would only re-parse and fail to embed).
func (h *RepoHandler) hasPendingEmbedding(ctx context.Context, repoID string) bool {
	info, err := h.sidecar.EmbedInfo(ctx)
	if err != nil || info.Exhausted {
		return false
	}

	pending, err := h.store.CountFilesPendingEmbedding(ctx, repoID, info.Model)
	if err != nil {
		h.logger.Warn("pending_embedding_check_failed", "repo_id", repoID, "error", err)
		return false
	}
	return pending > 0
}

func (h *RepoHandler) cloneExists(repoID string) bool {
	_, err := os.Stat(filepath.Join(repoDirFor(repoID), ".git"))
	return err == nil
}

// restoreCloneInBackground triggers a re-sync for a repo whose clone
// directory is missing (see checkAndEnqueueSync). Fire-and-forget: the
// caller answers the current request immediately.
func (h *RepoHandler) restoreCloneInBackground(repoID, repoURL string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := h.checkAndEnqueueSync(ctx, repoID, repoURL); err != nil {
			h.logger.Warn("clone_restore_enqueue_failed", "repo_id", repoID, "error", err)
		}
	}()
}

func generateRepoDirName() string {
	return fmt.Sprintf("repo_%d", time.Now().UnixNano())
}
