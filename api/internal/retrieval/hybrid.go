package retrieval

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type RetrievalResult struct {
	Symbol		string 		`json:"symbol"`
	FilePath    string      `json:"file_path"`
	Edges		[]GraphEdge `json:"edges"`
	Document    string       `json:"document"`
	Metadata    map[string]interface{} 		`json:"metadata"`
	Distance    float64		 `json:"distance"`
}

type GraphEdge struct {
	Caller       string      `json:"caller"`
	Callee       string		 `json:"callee"`
}


type HybridRetriever struct {
	store *store.Store
	sidecar  *sidecar.Client
	logger   *slog.Logger
}


func NewHybridRetriever(store *store.Store, sidecar *sidecar.Client, logger *slog.Logger) *HybridRetriever{

	return &HybridRetriever{
		store: store,
		sidecar: sidecar,
		logger: logger,
	}
}

func (r *HybridRetriever) ExpandSymbol(ctx context.Context, repoID, filePath, symbol string) ([]GraphEdge, error) {

	outgoing, err := r.store.GetOutgoingCalls(ctx, repoID, filePath, symbol)

	if err != nil {
		return nil, err
	}

	incoming, err := r.store.GetIncomingCalls(ctx, repoID, symbol)

	if err != nil {
		return nil, err
	}

	var edges []GraphEdge

	for _, e := range outgoing {

		edges = append(edges, GraphEdge{
			Caller: e.CallerSymbol,
			Callee: e.CalleeSymbol,
		})
	}

	for _, e := range incoming {
		edges = append(edges, GraphEdge{
			Caller: e.CallerSymbol,
			Callee: e.CalleeSymbol,
		})
	}

	r.logger.Info(
		"graph_expansion_stats",
		"symbol", symbol,
		"file_path", filePath,
		"outgoing_count", len(outgoing),
		"incoming_count", len(incoming),
	)
	return edges, nil
}

func (r *HybridRetriever) Search (ctx context.Context,repoID string,query string) ([]RetrievalResult, error) {

	searchResults, err := r.sidecar.Search(ctx, sidecar.SearchRequest{
		Query: query,
		RepoID: repoID,
		Limit: 3,
	})

	if err != nil {
		return nil, err
	}

	r.logger.Info(
		"semantic_search_complete",
		"query", query,
		"results", len(searchResults),
	)
	

	var results []RetrievalResult

	for _, sr := range searchResults {

		// fmt.Printf("Search result Metadata: %+v\n", sr.Metadata)
		// fmt.Printf("Document: %s\n", sr.Document)

		fmt.Println(sr.Metadata["file_path"])
		symbolRaw, ok := sr.Metadata["symbol"]
		if !ok {
			continue
		}
		symbol, ok := symbolRaw.(string)
		if !ok {
			continue
		}

		filePathRaw, ok := sr.Metadata["file_path"]
		if !ok {
			continue 
		}

		filePath, ok := filePathRaw.(string)
		if !ok {
			continue
		}

		r.logger.Info(
			"retrieved_symbol",
			"symbol", symbol,
			"file_path", filePath,
		)

		r.logger.Info(
			"graph_expansion_start",
			"symbol", symbol,
			"file_path", filePath,
		)

		//Graph Expansion Logic
		edges, err := r.ExpandSymbol(ctx, repoID, filePath, symbol)
		if err != nil {
			continue
		}

		r.logger.Info(
			"graph_expansion_complete",
			"symbol", symbol,
			"file_path", filePath,
			"edges_found", len(edges),
		)

		results = append(results, RetrievalResult{
			Symbol: symbol,
			FilePath: filePath,
			Document: sr.Document,
			Metadata: sr.Metadata,
			Distance: sr.Distance,
			Edges: edges,
		})
	}

	return results, nil
}