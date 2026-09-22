package answer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/architecture"
	"github.com/atharva-3105/KnowYourRepo/internal/metrics"
	"github.com/atharva-3105/KnowYourRepo/internal/rag"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
)

// RepoSyncer lets the service trigger a background re-ingestion check
// without depending on the ingestion/api packages directly - implemented
// by RepoHandler and injected at construction time. Ported unchanged from
// api/internal/agent/service.go.
type RepoSyncer interface {
	SyncIfStale(ctx context.Context, repoID string) error
}

const maxArchitectureListItems = 15

// Service is the single entry point into answering a repository question:
// always runs semantic search (which already auto-expands call-graph
// edges via HybridRetriever), and only conditionally adds an architecture
// overview when the question's phrasing calls for one. Replaces the old
// Planner -> HybridPlanner -> Executor -> Tool framework in api/internal/agent,
// which duplicated work HybridRetriever already does automatically or data
// (history) already passed separately - see
// docs/superpowers/specs/2026-09-07-agent-tool-simplification-design.md.
type Service struct {
	retriever           *retrieval.HybridRetriever
	architectureService *architecture.Service
	ragService          *rag.Service
	syncer              RepoSyncer
	logger              *slog.Logger
}

func NewService(
	retriever *retrieval.HybridRetriever,
	architectureService *architecture.Service,
	ragService *rag.Service,
	syncer RepoSyncer,
	logger *slog.Logger,
) *Service {
	return &Service{
		retriever:           retriever,
		architectureService: architectureService,
		ragService:          ragService,
		syncer:              syncer,
		logger:              logger,
	}
}

// Answer plans, executes, and answers a repository question. If the
// question implies the user wants the repo's latest state, a background
// re-ingestion check is triggered (fire-and-forget - the answer is still
// built from whatever's currently indexed; refreshing reports true so the
// caller can tell the user this answer might be slightly stale).
//
// The merged retrieval results are also returned (not just the final answer
// string) so the caller can surface them as structured citations - these are
// exactly the results that were fed into the LLM prompt for this answer.
func (s *Service) Answer(ctx context.Context, repoID, query, history string) (answer string, refreshing bool, results []retrieval.RetrievalResult, err error) {

	s.logger.Info("answer_service_started", "repo_id", repoID, "query", query)

	if WantsReingestion(query) {
		refreshing = true
		s.triggerBackgroundSync(repoID)
	}

	results, searchErr := s.retriever.Search(ctx, repoID, query)
	if searchErr != nil {
		// Not fatal - the lexical/call-graph lookup below doesn't depend on
		// the embedding provider at all, and can still ground a question
		// that names a real identifier even when semantic search is down.
		metrics.AgentToolExecutionsTotal.WithLabelValues("semantic", "failed").Inc()
		s.logger.Warn("answer_service_search_failed", "repo_id", repoID, "error", searchErr)
		results = nil
	} else {
		metrics.AgentToolExecutionsTotal.WithLabelValues("semantic", "success").Inc()
	}

	if lexicalResults, lexErr := s.retriever.LexicalSearch(ctx, repoID, query); lexErr != nil {
		metrics.AgentToolExecutionsTotal.WithLabelValues("lexical", "failed").Inc()
		s.logger.Warn("answer_service_lexical_search_failed", "repo_id", repoID, "error", lexErr)
	} else {
		metrics.AgentToolExecutionsTotal.WithLabelValues("lexical", "success").Inc()
		results = mergeResults(results, lexicalResults)
	}

	// Only genuinely fatal if semantic search failed AND lexical grounding
	// found nothing either - there's no context left to answer from at all.
	if searchErr != nil && len(results) == 0 {
		return "", refreshing, nil, searchErr
	}

	if WantsArchitectureOverview(query) {
		if overview, ovErr := s.architectureService.BuildSummary(ctx, repoID); ovErr != nil {
			// Not fatal - a request that already has real semantic results
			// shouldn't fail over a missed architecture overview.
			metrics.AgentToolExecutionsTotal.WithLabelValues("architecture", "failed").Inc()
			s.logger.Warn("answer_service_architecture_overview_failed", "repo_id", repoID, "error", ovErr)
		} else {
			metrics.AgentToolExecutionsTotal.WithLabelValues("architecture", "success").Inc()
			results = append(results, overviewAsResult(repoID, overview))
		}
	}

	s.logger.Info("answer_service_retrieval_complete", "repo_id", repoID, "results", len(results))

	answer, err = s.ragService.AnswerQuestion(ctx, query, history, results)
	if err != nil {
		s.logger.Error("answer_service_failed", "repo_id", repoID, "error", err)
		return "", refreshing, nil, err
	}

	s.logger.Info("answer_service_completed", "repo_id", repoID)

	return answer, refreshing, results, nil
}

// mergeResults appends extra's entries onto base, skipping any whose
// symbol+file_path already appears in base - so a lexical hit never
// duplicates a symbol semantic search already surfaced, but still adds
// genuinely new grounding (a call-graph edge to a symbol semantic search
// missed entirely) that base doesn't have.
func mergeResults(base, extra []retrieval.RetrievalResult) []retrieval.RetrievalResult {
	seen := make(map[string]bool, len(base))
	for _, r := range base {
		seen[r.Symbol+"|"+r.FilePath] = true
	}

	for _, r := range extra {
		key := r.Symbol + "|" + r.FilePath
		if seen[key] {
			continue
		}
		seen[key] = true
		base = append(base, r)
	}

	return base
}

func (s *Service) triggerBackgroundSync(repoID string) {
	if s.syncer == nil {
		return
	}

	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.syncer.SyncIfStale(syncCtx, repoID); err != nil {
			s.logger.Warn("answer_background_sync_failed", "repo_id", repoID, "error", err)
		}
	}()
}

// overviewAsResult builds the same text block the old ArchitectureTool
// produced, as a pseudo RetrievalResult so it flows into the LLM prompt
// through the same path as real search results. FilePath is set to the
// repo ID (not a real file) - callers building citations from these
// results must filter this one out by Symbol == "architecture_overview".
func overviewAsResult(repoID string, summary *architecture.Summary) retrieval.RetrievalResult {
	var b strings.Builder

	// The narrative summary/concepts are LLM-generated once at ingestion
	// (architecture.Service.GenerateOverview) and already shown on the
	// Overview page - this is the single best "what is this project" text
	// the system produces, and previously never reached Chat's own context
	// at all. Both are empty (not shown) for a repo where generation never
	// ran or failed - see architecture.Summary's own degrade-gracefully
	// convention.
	if summary.NarrativeSummary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", summary.NarrativeSummary)
	}

	fmt.Fprintf(&b, "Repository statistics: %d files, %d symbols, %d call edges.\n",
		summary.Statistics.FileCount, summary.Statistics.SymbolCount, summary.Statistics.CallEdges)
	fmt.Fprintf(&b, "Languages: %s\n", strings.Join(summary.Languages, ", "))

	if len(summary.Concepts) > 0 {
		b.WriteString("Notable concepts:\n")
		for i, c := range summary.Concepts {
			if i >= maxArchitectureListItems {
				break
			}
			fmt.Fprintf(&b, "- %s: %s\n", c.Term, c.Explanation)
		}
	}

	b.WriteString("EntryPoints:\n")
	for i, ep := range summary.EntryPoints {
		if i >= maxArchitectureListItems {
			break
		}
		fmt.Fprintf(&b, "- %s (%s) in %s\n", ep.Name, ep.Type, ep.FilePath)
	}

	b.WriteString("Components:\n")
	for i, c := range summary.Components {
		if i >= maxArchitectureListItems {
			break
		}
		fmt.Fprintf(&b, "- %s (%s) in %s\n", c.Name, c.Type, c.FilePath)
	}

	return retrieval.RetrievalResult{
		Symbol:   "architecture_overview",
		FilePath: repoID,
		Document: b.String(),
	}
}
