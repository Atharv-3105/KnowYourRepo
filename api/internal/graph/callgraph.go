package graph 

import (
	sitter "github.com/smacker/go-tree-sitter"
)

type CallEdge struct {
	Caller string // The function which is referencing another function 
	Callee string //The function which is being referenced
}


func ExtractGoCallGraph(root *sitter.Node, source []byte) []CallEdge{

	var edges []CallEdge
	var currentFunction string 

	var walk func(node *sitter.Node)

	walk = func(node *sitter.Node) {

		if node == nil {
			return 
		}

		//Track current function
		if node.Type() == "function_declaration" {

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				currentFunction = nameNode.Content(source)
			}
		}

		//Detect function calls
		if node.Type() == "call_expression" {

			functionNode := node.ChildByFieldName("function") 

			if functionNode != nil && currentFunction != "" {

				callee := functionNode.Content(source)

				edges = append(edges,
							   CallEdge{
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

