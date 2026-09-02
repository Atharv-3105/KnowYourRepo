package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"path/filepath"

	"github.com/atharva-3105/KnowYourRepo/internal/agent"
	"github.com/atharva-3105/KnowYourRepo/internal/agent/tools"
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
	IngestionWorkerCount = 2
	IngestionQueueSize   = 16
)

type RepoHandler struct {
	logger              *slog.Logger
	store               *store.Store
	sidecar             *sidecar.Client
	parser              *ingestion.TreeSitterParser
	extractor           *graph.Extractor
	cloner              *ingestion.Cloner
	walker              *ingestion.Walker
	hybridRetriever     *retrieval.HybridRetriever
	contextBuilder      *contextbuilder.Builder
	ragService          *rag.Service
	chatStore           *chat.Store
	architectureService *architecture.Service
	agentService        *agent.Service
	workerPool          *worker.Pool
}

func NewRepoHandler(
	logger  *slog.Logger,
	store   *store.Store,
	sidecar *sidecar.Client,
) *RepoHandler {

	builder := contextbuilder.NewBuilder(logger)
	architectureAnalyzer := architecture.NewAnalyzer(logger, store)
	architectureService := architecture.NewService(logger, architectureAnalyzer)

	hybridRetriever := retrieval.NewHybridRetriever(store, sidecar, logger)
	ragService := rag.NewService(builder, sidecar, logger)

	agentTools := map[agent.ToolName]agent.Tool{
		agent.ToolSemantic:     tools.NewSemanticTool(hybridRetriever),
		agent.ToolGraph:        tools.NewGraphTool(store),
		agent.ToolMemory:       tools.NewMemoryTool(),
		agent.ToolArchitecture: tools.NewArchitectureTool(architectureService),
	}

	fallbackPlanner := agent.NewPlanner()
	hybridPlanner := agent.NewHybridPlanner(sidecar, fallbackPlanner, logger)
	executor := agent.NewExecutor(agentTools, logger)

	h := &RepoHandler{
		logger:              logger,
		store:               store,
		sidecar:             sidecar,
		parser:              ingestion.NewParser(logger),
		extractor:           graph.NewExtractor(logger),
		cloner:              ingestion.NewCloner(logger),
		walker:              ingestion.NewWalker(logger),
		hybridRetriever:     hybridRetriever,
		contextBuilder:      builder,
		ragService:          ragService,
		chatStore:           chat.NewStore(),
		architectureService: architectureService,
	}

	// agentService needs h as its RepoSyncer (for freshness-triggered
	// background sync), so it's constructed after h exists - same
	// deferred-wiring pattern as workerPool below.
	h.agentService = agent.NewService(hybridPlanner, executor, ragService, h, logger)

	workerPool := worker.NewPool(IngestionWorkerCount, IngestionQueueSize, h.handleIngestionJob, logger)
	workerPool.Start(context.Background())

	h.workerPool = workerPool

	return h
}

func(h *RepoHandler) CreateRepo(c *gin.Context) {

	var req CreateRepoRequest

	if err := c.ShouldBindJSON(&req); err != nil{
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return 
	}

	ctx := c.Request.Context()

	//Deduplication by URL - if this repo was already ingested, refresh the existing repo_id instead of cloning a brand new one
	existingRepo, err := h.store.GetRepositoryByURL(ctx, req.RepoURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	repoID := ""

	//If the repo is already present, reuse its ID - otherwise create a fresh one
	if existingRepo != nil {
		repoID = existingRepo.ID

		h.logger.Info("repo_already_ingested_refreshing", "repo_id", repoID, "repo_url", req.RepoURL)
	} else {

		//Create the complete process of Fresh RepoID creation
		repoID = generateRepoDirName()

		//Insert the repository into the DB
		if err := h.store.InsertRepository(ctx, store.Repository{
			ID:			repoID,
			RepoURL:    req.RepoURL,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	//Generate a Job for Ingestion workers - runs for both the existing-repo
	//refresh path and the brand-new-repo path
	if err := h.store.InsertJob(ctx, repoID, req.RepoURL); err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !h.workerPool.Enqueue(repoID) {

		errMsg := "ingestion queue is full, try again shortly"

		if updateErr := h.store.UpdateJobStatus(ctx,repoID, store.JobStatusFailed, &errMsg); updateErr != nil{

			h.logger.Error("failed to mark job failed", "job_id", repoID, "error", updateErr)
		}

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": errMsg})
		return
	}

	h.logger.Info("ingestion_job_queued", "repo_id", repoID, "repo_url", req.RepoURL)

	c.JSON(http.StatusAccepted, CreateRepoResponse{
		Success: true,
		Message: "ingestion queued",
		RepoID:	repoID,
	})
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
		metrics.IngestionJobDuration.WithLabelValues("failed").Observe(time.Since(start).Seconds())

		msg := err.Error()
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

	repoID := jobID
	repoURL := job.RepoURL

	repoDir := filepath.Join("..", "data", "repos", repoID)

	h.logger.Info("starting repo ingestion", "repo_url", repoURL)

	//Clone if this is the first time, sync (fetch + hard reset) if we've
	//already cloned this repo before.
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {

		if err := h.cloner.CloneRepo(ctx, repoURL, repoDir); err != nil {
			return fmt.Errorf("clone failed: %w", err)
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
	seenPaths := make(map[string]bool)

	var skipped, processed int

	//Process the files
	for _, file := range files {

		seenPaths[file.Path] = true

		hash, err := ingestion.HashFile(file.Path)
		if err != nil {
			h.logger.Warn("failed to hash file", "path", file.Path, "error", err)
			continue
		}

		prior, hadPrior := existing[file.Path]

		if hadPrior && prior.Hash == hash {
			//Unchanged since last ingestion - skip parsing/extraction/embedding entirely
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

		embedItems = append(embedItems, items...)
		repoIR.Files = append(repoIR.Files, irFile)
		processed++
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

	h.logger.Info("incremental_diff_complete", "processed", processed, "skipped", skipped, "deleted", len(existing)-len(seenPaths))

	if len(embedItems) > 0 {

		h.logger.Info("sending batch embeddings", "count", len(embedItems))

		if err := h.sidecar.Embed(ctx, sidecar.EmbedBatchRequest{Items: embedItems}); err != nil {
			return fmt.Errorf("batch embedding failed: %w", err)
		}
	}

	//Save IR File Representation
	irPath := filepath.Join(repoDir, "repository_ir.json")

	if err := representation.SaveRepository(repoIR, irPath); err != nil {
		h.logger.Error("failed to save repository IR", "error", err)
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

	remoteSHA, err := ingestion.RemoteHeadSHA(ctx, repoURL)
	if err != nil {
		return false, fmt.Errorf("failed to check remote head: %w", err)
	}

	localSHA, err := h.store.GetLastCommitSHA(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("failed to get last known commit sha: %w", err)
	}

	if remoteSHA == localSHA {
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

	h.logger.Info("repo_sync_enqueued", "repo_id", repoID, "remote_sha", remoteSHA, "local_sha", localSHA)

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	enqueued, err := h.checkAndEnqueueSync(ctx, repo.ID, repo.RepoURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !enqueued {
		c.JSON(http.StatusOK, gin.H{"status": "up_to_date"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "sync_queued", "repo_id": repo.ID})
}

// SyncIfStale implements agent.RepoSyncer - looks up the repo's URL and
// runs the same cheap staleness check + background enqueue used by the
// on-demand /repos/:id/sync endpoint. Called by the agent when a chat
// question implies the user wants the repo's latest state.
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


func generateRepoDirName() string {
	return fmt.Sprintf("repo_%d", time.Now().UnixNano())
}