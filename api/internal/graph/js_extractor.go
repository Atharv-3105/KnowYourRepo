package graph 

import (
	"log/slog"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractJSSymbols(
	root *sitter.Node,
	source []byte,
	logger *slog.Logger,
) []Symbol {

	var symbols []Symbol

	var walk func(node *sitter.Node)

	walk = func(node *sitter.Node) {

		if node.Type() == "function_declaration" || node.Type() == "method_declaration" {

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				symbolName := nameNode.Content(source)
				symbols = append(symbols, Symbol {
					Name:   symbolName,
					Type:   "function",
					StartLine:  int(node.StartPoint().Row) + 1,
					EndLine:	int(node.EndPoint().Row) + 1,
				})

				logger.Debug("extracted js symbol", "name", symbolName, "type", "function")
			}else{
				logger.Warn("found function declaration without a name node")
			}
		}

		if node.Type() == "class_declaration" {
			
			nameNode := node.ChildByFieldName("name") 

			if nameNode != nil {
				symbolName := nameNode.Content(source)
				symbols = append(symbols, Symbol{
					Name:  symbolName,
					Type:  "class",
					StartLine: int(node.StartPoint().Row) + 1,
					EndLine:    int(node.EndPoint().Row) + 1,
				})

				logger.Debug("extracted js symbol", "name", symbolName, "type", "class")
			}else{
				logger.Warn("found class declaration without a name node.")
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return symbols 
}