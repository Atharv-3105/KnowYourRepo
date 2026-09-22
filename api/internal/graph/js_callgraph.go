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

		// Anonymous function/arrow expression not already named via the
		// variable-declarator case above (e.g. an IIFE, or a callback
		// passed inline) - see anonymousFuncName's doc comment in
		// callgraph.go for why this needs a synthetic name rather than its
		// raw source text. The variable_declarator check above runs on the
		// *parent* node and only reaches this literal via the inherited
		// caller parameter - checking the immediate parent directly here
		// (rather than comparing nextCaller/caller, which can't tell
		// "already named by my parent" apart from "not named at all") is
		// what actually distinguishes `const hello = () => {...}` (already
		// named "hello", must not be overwritten) from a bare IIFE or
		// inline callback (never named, needs the synthetic name).
		if node.Type() == "arrow_function" || node.Type() == "function_expression" {
			parent := node.Parent()
			var alreadyNamed bool
			if parent != nil && parent.Type() == "variable_declarator" {
				if valueNode := parent.ChildByFieldName("value"); valueNode != nil {
					alreadyNamed = valueNode.StartByte() == node.StartByte()
				}
			}
			if !alreadyNamed {
				nextCaller = anonymousFuncName(node)
			}
		}

		//Function calls
		if node.Type() == "call_expression" {

			functionNode := node.ChildByFieldName("function")

			if functionNode != nil && caller != "" {

				var callee string
				unwrapped := unwrapParens(functionNode)
				if unwrapped.Type() == "arrow_function" || unwrapped.Type() == "function_expression" {
					callee = anonymousFuncName(unwrapped)
				} else {
					callee = functionNode.Content(source)
				}

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