package retrieval

import (
	"context"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type GraphRetriever struct {
	store  *store.Store
}

func NewGraphRetriever(store *store.Store) *GraphRetriever {

	return &GraphRetriever{
		store: store,
	}
}

func (r *GraphRetriever) ExpandContext(ctx context.Context, symbol string) ([]store.CallEdge, error) {

	outgoing, err := r.store.GetOutgoingCalls(ctx, symbol)

	if err != nil {
		return nil, err
	}

	incoming, err := r.store.GetIncomingCalls(ctx, symbol)
	
	if err != nil {
		return nil, err
	}

	var edges []store.CallEdge

	edges = append(edges, outgoing...)
	edges = append(edges, incoming...)

	return edges, nil
}