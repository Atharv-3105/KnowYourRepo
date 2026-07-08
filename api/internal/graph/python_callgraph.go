package graph 

import (
	sitter "github.com/smacker/go-tree-sitter"
)


func ExtractPythonCallGraph(root *sitter.Node, source []byte) []CallEdge {

	var edges []CallEdge

	var walk func(node *sitter.Node, caller string)

	walk = func(node *sitter.Node, caller string) {

		if node == nil {
			return 
		}

		nextCaller := caller

		//Track current function
		if node.Type() == "function_definition" {

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				nextCaller = nameNode.Content(source)
			}
		}

		//Detect function callss
		if node.Type() == "call" {

			functionNode := node.ChildByFieldName("function")

			if functionNode != nil && caller != "" {

				callee := functionNode.Content(source)

				edges = append(edges,
							   CallEdge{
									Caller: caller,
									Callee: callee,
							   })
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), nextCaller)
		}
	}

	walk(root, "")

	return edges
}