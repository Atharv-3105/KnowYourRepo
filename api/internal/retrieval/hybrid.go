package retrieval

import (
	"context"
	"fmt"

	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type RetrievalResult struct {
	Symbol		string 		`json:"symbol"`
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
}


func NewHybridRetriever(store *store.Store, sidecar *sidecar.Client) *HybridRetriever{

	return &HybridRetriever{
		store: store,
		sidecar: sidecar,
	}
}

func (r *HybridRetriever) ExpandSymbol(ctx context.Context, symbol string) ([]GraphEdge, error) {

	outgoing, err := r.store.GetOutgoingCalls(ctx, symbol)

	if err != nil {
		return nil, err
	}

	incoming, err := r.store.GetIncomingCalls(ctx, symbol)

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

	return edges, nil
}

func (r *HybridRetriever) Search (ctx context.Context,repoID string,query string) ([]RetrievalResult, error) {

	searchResults, err := r.sidecar.Search(ctx, sidecar.SearchRequest{
		Query: query,
		RepoID: repoID,
		Limit: 5,
	})

	if err != nil {
		return nil, err
	}

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

		edges, err := r.ExpandSymbol(ctx, symbol)
		if err != nil {
			continue
		}

		results = append(results, RetrievalResult{
			Symbol: symbol,
			Document: sr.Document,
			Metadata: sr.Metadata,
			Distance: sr.Distance,
			Edges: edges,
		})
	}

	return results, nil
}