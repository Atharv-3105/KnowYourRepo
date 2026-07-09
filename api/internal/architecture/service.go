package architecture

import (

	"context"
	"log/slog"
)

type Service struct {
	logger *slog.Logger 
	analyzer *Analyzer
}

func NewService(logger *slog.Logger, analyzer  *Analyzer) *Service {

	return &Service{
		logger: logger, 
		analyzer: analyzer,
	}
}


//This function analyzes a repo and returns its architectural overview
func(s *Service) BuildSummary(ctx context.Context, repoID string) (*Summary, error){

	s.logger.Info("architecture_service_started", "repo_id", repoID)

	summary, err := s.analyzer.AnalyzeRepository(ctx, repoID)

	if err != nil {
		
		s.logger.Error("architecture_service_failed", "repo_id", repoID, "error", err)

		return nil, err 
	}

	s.logger.Error("architecture_service_completed",
				   "repo_id", repoID, "files", summary.Statistics.FileCount,
				   "symbols", summary.Statistics.SymbolCount, "components", len(summary.Components),
				   "entrypoints", len(summary.EntryPoints))		


	return summary, nil 
}