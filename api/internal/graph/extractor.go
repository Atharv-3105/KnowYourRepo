package graph

import(
	"fmt"
	"log/slog"

	sitter "github.com/smacker/go-tree-sitter"
)

type Extractor struct{logger *slog.Logger}

func NewExtractor(logger *slog.Logger) *Extractor{
	return &Extractor{
		logger: logger,
	}
}

//Function ExtractSymbol which extracts symbols from ASI root
func (e *Extractor) ExtractSymbols(
	root *sitter.Node,
	source []byte, 
	language string,
) ([]Symbol, error) {

	e.logger.Debug("starting symbol extraction", "language", language)

	var symbols []Symbol
	// var err error

	switch language {
	case "go":
		symbols =  extractGoSymbols(root, source, e.logger)
	case "python":
		symbols =  extractPythonSymbols(root, source, e.logger) 
	case "javascript", "typescript":
		symbols =  extractJSSymbols(root, source, e.logger)

	default:
		e.logger.Warn("unsupported language for extraction", "language", language)
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	e.logger.Info("extraction complete", "language", language, "symbols_found", len(symbols))

	return symbols, nil 
}
