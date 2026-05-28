package graph

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func ExtractJSCallGraph(root *sitter.Node, source []byte) []CallEdge {

	var edges []CallEdge

	var currentFunction string 

	var walk func(node *sitter.Node)

	walk = func(node *sitter.Node) {

		if node == nil {
			return 
		}

		//Common JS Function
		if node.Type() == "function_declaration" {
			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				currentFunction = nameNode.Content(source)
			}
		}

		//Variable-assigned arrow function
		if node.Type() == "lexical_declaration" {

			for i := 0; i < int(node.ChildCount()); i++ {

				child := node.Child(i)

				if child.Type() == "variable_declarator" {

					nameNode := child.ChildByFieldName("name")

					valueNode := child.ChildByFieldName("value")

					if nameNode != nil && valueNode != nil && valueNode.Type() == "arrow_function" {
						currentFunction = nameNode.Content(source)
					}
				}
			}
		}

		//Function calls
		if node.Type() == "call_expression" {

			functionNode := node.ChildByFieldName("function")

			if functionNode != nil && currentFunction != "" {
				callee := functionNode.Content(source)

				edges = append(edges, CallEdge{
						Caller: currentFunction,
						Callee: callee,
				})
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)

	return edges

}