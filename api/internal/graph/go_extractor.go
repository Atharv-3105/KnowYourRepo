package graph

import (
	"log/slog"
	sitter "github.com/smacker/go-tree-sitter"
)


func extractGoSymbols(root *sitter.Node, source []byte, logger *slog.Logger) []Symbol {
	var symbols []Symbol

	var walk func(node *sitter.Node)

	walk = func(node *sitter.Node) {
		if node == nil {
			return 
		}

		if node.Type() == "function_declaration" || node.Type() == "method_declaration" {

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				//Get the name of the symbol
				symbolName := nameNode.Content(source)

				symbols = append(symbols, Symbol{
					Name: 	symbolName,
					Type:	"function",
					StartLine: int(node.StartPoint().Row) + 1,
					EndLine:   int(node.EndPoint().Row) + 1,
				})

				logger.Debug("extracted go symbol", "name", symbolName, "type", "function")
			}else {
				//This is useful to know exactly which node is being failed to parse 
				logger.Warn("found function declaration without a name node")
			}
		}
		
		//Iteratively walk all the children of current-node
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)

	return symbols
}