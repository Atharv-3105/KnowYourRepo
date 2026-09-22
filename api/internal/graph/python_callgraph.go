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
		} else if node.Type() == "lambda" {
			// Anonymous lambda (e.g. `(lambda: helper())()`) - see
			// anonymousFuncName's doc comment in callgraph.go for why this
			// needs a synthetic name rather than its raw source text.
			nextCaller = anonymousFuncName(node)
		}

		//Detect function callss
		if node.Type() == "call" {

			functionNode := node.ChildByFieldName("function")

			if functionNode != nil && caller != "" {

				var callee string
				if unwrapped := unwrapParens(functionNode); unwrapped.Type() == "lambda" {
					callee = anonymousFuncName(unwrapped)
				} else {
					callee = functionNode.Content(source)
				}

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