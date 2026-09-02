package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/rag"
)

// RepoSyncer lets the agent trigger a background re-ingestion check
// without depending on the ingestion/api packages directly - implemented
// by RepoHandler and injected at construction time.
type RepoSyncer interface {
	SyncIfStale(ctx context.Context, repoID string) error
}

//This is the Entry-point into the agentic Pipeline:
//HybridPlanner -> Executor(TOOLS) -> ContextBuilder/RAG -> LLM For Chat
type Service struct {
	planner *HybridPlanner
	executor *Executor
	ragService *rag.Service
	syncer RepoSyncer
	logger *slog.Logger
}


func NewService(planner *HybridPlanner, executor *Executor, ragService *rag.Service, syncer RepoSyncer, logger *slog.Logger) *Service {
	return &Service{
		planner:    planner,
		executor:   executor,
		ragService: ragService,
		syncer:     syncer,
		logger:     logger,
	}
}

// Answer plans, executes, and answers a repository question. If the
// question implies the user wants the repo's latest state, a background
// re-ingestion check is triggered (fire-and-forget - the answer is still
// built from whatever's currently indexed; refreshing reports true so the
// caller can tell the user this answer might be slightly stale).
func (s *Service) Answer(ctx context.Context, repoID, query, history string) (answer string, plan []ToolName, refreshing bool, err error) {

	s.logger.Info("agent_service_started", "repo_id", repoID, "query", query)

	if WantsReingestion(query) {
		refreshing = true
		s.triggerBackgroundSync(repoID)
	}

	plan = s.planner.Plan(ctx, query, history)

	s.logger.Info("agent_plan_selected", "repo_id", repoID, "tools", plan)

	results := s.executor.Execute(ctx, plan, ToolRequest{
		RepoID:  repoID,
		Query:   query,
		History: history,
	})

	s.logger.Info("agent_execution_complete", "repo_id", repoID, "merged_results", len(results))

	answer, err = s.ragService.AnswerQuestion(ctx, query, history, results)
	if err != nil {
		s.logger.Error("agent_service_failed", "repo_id", repoID, "error", err)
		return "", plan, refreshing, err
	}

	s.logger.Info("agent_service_completed", "repo_id", repoID)

	return answer, plan, refreshing, nil
}

func (s *Service) triggerBackgroundSync(repoID string) {

	if s.syncer == nil {
		return
	}

	go func() {

		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.syncer.SyncIfStale(syncCtx, repoID); err != nil {
			s.logger.Warn("agent_background_sync_failed", "repo_id", repoID, "error", err)
		}
	}()
}