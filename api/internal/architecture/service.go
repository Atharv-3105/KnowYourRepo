package architecture

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type Service struct {
	logger   *slog.Logger
	analyzer *Analyzer
	store    *store.Store
	sidecar  *sidecar.Client
}

func NewService(logger *slog.Logger, analyzer *Analyzer, store *store.Store, sidecar *sidecar.Client) *Service {

	return &Service{
		logger:   logger,
		analyzer: analyzer,
		store:    store,
		sidecar:  sidecar,
	}
}

//This function analyzes a repo and returns its architectural overview
func (s *Service) BuildSummary(ctx context.Context, repoID string) (*Summary, error) {

	s.logger.Info("architecture_service_started", "repo_id", repoID)

	summary, err := s.analyzer.AnalyzeRepository(ctx, repoID)

	if err != nil {

		s.logger.Error("architecture_service_failed", "repo_id", repoID, "error", err)

		return nil, err
	}

	s.logger.Info("architecture_service_completed",
		"repo_id", repoID, "files", summary.Statistics.FileCount,
		"symbols", summary.Statistics.SymbolCount, "components", len(summary.Components),
		"entrypoints", len(summary.EntryPoints))

	return summary, nil
}

const maxRepresentativeSymbols = 30

// GenerateOverview builds the LLM prompt inputs from what's already in the
// store (entrypoints, directory structure, a sample of symbols), calls the
// sidecar's /generate-overview route, and persists the result. Called once
// per ingestion/sync job, after call-graph extraction has completed (see
// ingestRepository in api/internal/api/repos.go). readmeText may be empty -
// generation still runs, just without that section of the prompt.
func (s *Service) GenerateOverview(ctx context.Context, repoID, readmeText string) error {

	files, err := s.store.GetFiles(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to load files for overview generation: %w", err)
	}

	symbols, err := s.store.GetSymbols(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to load symbols for overview generation: %w", err)
	}

	entrypoints := DetectEntrypoints(files, symbols)

	overviewEntrypoints := make([]sidecar.OverviewEntrypoint, 0, len(entrypoints))
	for _, ep := range entrypoints {
		overviewEntrypoints = append(overviewEntrypoints, sidecar.OverviewEntrypoint{
			Name:     ep.Name,
			FilePath: ep.FilePath,
			Language: ep.Language,
		})
	}

	resp, err := s.sidecar.GenerateOverview(ctx, sidecar.GenerateOverviewRequest{
		ReadmeText:            readmeText,
		Entrypoints:           overviewEntrypoints,
		DirectoryStructure:    topLevelDirs(files, repoID),
		RepresentativeSymbols: representativeSymbolStrings(symbols, maxRepresentativeSymbols),
	})
	if err != nil {
		return fmt.Errorf("overview generation request failed: %w", err)
	}

	concepts := make([]store.Concept, 0, len(resp.Concepts))
	for _, c := range resp.Concepts {
		concepts = append(concepts, store.Concept{Term: c.Term, Explanation: c.Explanation})
	}

	if err := s.store.SaveOverview(ctx, repoID, resp.NarrativeSummary, concepts); err != nil {
		return fmt.Errorf("failed to save generated overview: %w", err)
	}

	s.logger.Info("overview_generation_completed", "repo_id", repoID, "concepts", len(concepts))

	return nil
}

// repoRelativeSegments splits a stored file path (the clone directory's own
// absolute on-disk location - see the Global Constraints note in this
// plan/the design spec - not a clean repo-relative path) into segments after
// the repoID, mirroring frontend/src/lib/path.ts's repoRelativePath.
func repoRelativeSegments(filePath, repoID string) []string {
	segments := strings.FieldsFunc(filePath, func(r rune) bool { return r == '\\' || r == '/' })

	idx := -1
	for i, seg := range segments {
		if seg == repoID {
			idx = i
			break
		}
	}

	if idx >= 0 && idx+1 < len(segments) {
		return segments[idx+1:]
	}
	return segments
}

// topLevelDirs returns the deduplicated, sorted set of top-level directory
// names in the repo (relative to the repo root), for the overview prompt's
// "directory structure" section.
func topLevelDirs(files []store.ArchitectureFile, repoID string) []string {
	seen := make(map[string]struct{})

	for _, f := range files {
		rel := repoRelativeSegments(f.Path, repoID)
		dir := "(root)"
		if len(rel) > 1 {
			dir = rel[0]
		}
		seen[dir] = struct{}{}
	}

	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	return dirs
}

// representativeSymbolStrings caps how many symbols reach the overview
// prompt, formatted as "name (type)" - token-budget hygiene, see the design
// spec's Error handling section.
func representativeSymbolStrings(symbols []store.ArchitectureSymbol, limit int) []string {
	out := make([]string, 0, limit)

	for i, sym := range symbols {
		if i >= limit {
			break
		}
		out = append(out, fmt.Sprintf("%s (%s)", sym.Name, sym.Type))
	}

	return out
}
