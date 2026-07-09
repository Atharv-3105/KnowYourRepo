package architecture

import (
	"context"
	"log/slog"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)


type Analyzer struct {
	logger   *slog.Logger
	store    *store.Store
}


func NewAnalyzer(logger *slog.Logger, store *store.Store) *Analyzer {

	return &Analyzer{
		logger:		logger, 
		store:		store,
	}
}

func(a *Analyzer) AnalyzeRepository(ctx context.Context, repoID string) (*Summary, error) {

	a.logger.Info("architecture_analysis_started", "repo_id:", repoID)

	files, err := a.store.GetFiles(ctx, repoID)
	if err != nil {
		return nil, err
	}

	symbols, err := a.store.GetSymbols(ctx, repoID) 
	if err != nil{
		return nil, err
	}

	callEdges, err := a.store.GetCallEdges(ctx, repoID)
	if err != nil{
		return nil, err
	}

	languages, err := a.store.GetLanguages(ctx, repoID)
	if err != nil{
		return nil, err
	}

	a.logger.Info("architecture_repo_loaded", "repo_id", repoID, "files", len(files), "symbols", len(symbols), "call_edges", len(callEdges), "languages", len(languages))

	components := DetectComponents(files, symbols)

	a.logger.Info("architecture_components_detected", "count", len(components))

	//=======Detect Entrypoints===========
	entrypoints := DetectEntrypoints(files, symbols)

	a.logger.Info("architecture_entrypoints_detected", "count", len(entrypoints))

	//=========Build the Statistics=========
	stats := Statistics{
		FileCount: 		len(files),
		SymbolCount:    len(symbols),
		CallEdges:      len(callEdges),
	}

	summary := &Summary{
		RepoID:  		repoID,
		Statistics:     stats,
		Languages:      languages,
		EntryPoints:    entrypoints,
		Components:     components,	
	}

	a.logger.Info("architecture_analysis_complete", "repo_id", repoID, "files", stats.FileCount, "symbols", stats.SymbolCount, "components", len(summary.Components), "entrypoints", len(summary.EntryPoints))

	return summary, nil 
}