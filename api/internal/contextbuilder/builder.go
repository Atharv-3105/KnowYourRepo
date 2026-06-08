package contextbuilder

import( 
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"log/slog"
)

type Builder struct{
	logger *slog.Logger
}

func NewBuilder(logger *slog.Logger) *Builder {
	return &Builder{
		logger: logger, 
	}
}

func (b *Builder) Build(query string, results []retrieval.RetrievalResult) ContextPackage {

	const (
		MaxDocumentChars = 1200
		MaxCallsPerNode = 10
	)

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
		//Limit the CallsPerNode and remove Duplicate Calls
		node.Calls = DeduplicateCalls(node.Calls)
		node.Calls = LimitCalls(node.Calls, MaxCallsPerNode)

		b.logger.Info(
			"context_node",
			"symbol", result.Symbol,
			"document_size", len(result.Document),
			"compressed_size", len(TruncateDocument(result.Document, MaxDocumentChars)),
			"calls", len(node.Calls),
		)
		ctx.Entries = append(ctx.Entries, ContextNode{
			Symbol:   result.Symbol,
			Document:  TruncateDocument(result.Document, MaxDocumentChars),
			Calls:   node.Calls,
		})
	}

	var totalChars int 
	for _, e := range ctx.Entries{
		totalChars += len(e.Document)
	}
	b.logger.Info(
		"context_built", 
		"query", query, 
		"entries", len(ctx.Entries),
		"total_chars", totalChars,
	)
	return ctx
}