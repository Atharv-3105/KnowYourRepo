package contextbuilder

import "github.com/atharva-3105/KnowYourRepo/internal/retrieval"

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Build(query string, results []retrieval.RetrievalResult) ContextPackage {

	ctx := ContextPackage{
		Query: query,
	}

	for _, result := range results{

		node := ContextNode{
			Symbol: result.Symbol,
			Document: result.Document,
		}

		for _, edge := range result.Edges {

			if edge.Caller == result.Symbol {

				node.Calls = append(node.Calls, edge.Callee)
			}
		}

		ctx.Entries = append(ctx.Entries, node)
	}


	return ctx
}