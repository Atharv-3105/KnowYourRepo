package graph 

import (
	sitter "github.com/smacker/go-tree-sitter"
	"log/slog"
)

func extractPythonSymbols(
	root *sitter.Node, 
	source []byte,
	logger *slog.Logger,
) []Symbol {

	var symbols []Symbol 

	var walk func(node *sitter.Node)

	walk = func(node *sitter.Node) {

		if node.Type() == "function_definition" || node.Type() == "method_definition"{

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				symbolName := nameNode.Content(source)
				symbols = append(symbols, Symbol{
					Name:  symbolName,
					Type:  "function",
					StartLine:  int(node.StartPoint().Row) + 1,
					EndLine:    int(node.EndPoint().Row) + 1,
				})

				logger.Debug("extracted python symbol", "name", symbolName, "type", "function")
			}else{
				logger.Warn("found function definition without name node")
			}
		}

		if node.Type() == "class_definition" {

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				symbolName := nameNode.Content(source)
				symbols = append(symbols, Symbol{
					Name:    symbolName,
					Type:    "class",
					StartLine:  int(node.StartPoint().Row) + 1,
					EndLine:    int(node.EndPoint().Row) + 1,
				})

				logger.Debug("extracted python symbol", "name", symbolName, "type", "function")
			}else{
				logger.Warn("found function definition without name node")
			}
		}
		
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)

	return symbols 
}