package graph

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func ExtractJSCallGraph(root *sitter.Node, source []byte) []CallEdge {

	var edges []CallEdge

	var walk func(node *sitter.Node, caller string)

	walk = func(node *sitter.Node, caller string) {

		if node == nil {
			return 
		}

		nextCaller := caller

		//Common JS Function
		if node.Type() == "function_declaration" || node.Type() == "method_definition" {
			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				nextCaller = nameNode.Content(source)
			}
		}

		//Variable-assigned arrow function
		if node.Type() == "variable_declarator" {
			nameNode := node.ChildByFieldName("name")
			valueNode := node.ChildByFieldName("value")

			if nameNode != nil && valueNode != nil && valueNode.Type() == "arrow_function" {
				nextCaller = nameNode.Content(source)
			}
		}

		//Function calls
		if node.Type() == "call_expression" {

			functionNode := node.ChildByFieldName("function")

			if functionNode != nil && caller != "" {
				callee := functionNode.Content(source)

				edges = append(edges, CallEdge{
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