package agent

import (
	"context"
	"log/slog"

	"github.com/atharva-3105/KnowYourRepo/internal/rag"
)

//This is the Entry-point into the agentic Pipeline:
//HybridPlanner -> Executor(TOOLS) -> ContextBuilder/RAG -> LLM For Chat
type Service struct {
	planner *HybridPlanner
	executor *Executor
	ragService *rag.Service
	logger *slog.Logger
}


func NewService(planner *HybridPlanner, executor *Executor, ragService *rag.Service, logger *slog.Logger) *Service{
	return &Service{
		planner: planner,
		executor: executor,
		ragService: ragService,
		logger: logger,
	}
}

//Answer method plans, executes and answers an user query, returning the list of the tools that were selected
//alongside the final answer so callers can surface the agent's reasoning if desired.

func(s *Service) Answer(ctx context.Context, repoID, query, history string) (string, []ToolName, error){

	s.logger.Info("agent_service started", "repo_id", repoID, "query", query)

	plan := s.planner.Plan(ctx, query, history)

	s.logger.Info("agent_plan_started","repo_id", repoID, "tools", plan)

	//Get the merged tools based on the plan decided so RAG can be DONE on the results
	results := s.executor.Execute(ctx, plan, ToolRequest{
		RepoID: repoID,
		Query: query,
		History: history,
	})

	s.logger.Info("agent_execution_complete", "repo_id", repoID, "merged_results", results)

	answer, err := s.ragService.AnswerQuestion(ctx, query, history, results)
	if err != nil {
		s.logger.Error("agent_service_failed", "repo_id", repoID, "error", err)
		return "", plan, err 
	}

	s.logger.Info("agent_service_completed", "repo_id", repoID)

	return answer, plan, nil 
}