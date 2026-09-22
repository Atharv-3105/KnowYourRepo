package graph

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

type CallEdge struct {
	Caller string // The function which is referencing another function
	Callee string //The function which is being referenced
}

// anonymousFuncName gives an immediately-invoked or passed-as-value function
// literal a short, bounded synthetic name instead of using its raw source
// text (which for a closure of any real size becomes a multi-line blob as
// a "symbol name" - previously caused a real Groq 413 in production, and
// separately meant every call inside the closure got misattributed to
// whatever named function textually enclosed it, since there was nothing to
// set as the current caller). Line number keeps distinct literals in the
// same file distinguishable without needing the full body.
func anonymousFuncName(node *sitter.Node) string {
	return fmt.Sprintf("<anonymous:%d>", node.StartPoint().Row+1)
}

// unwrapParens drills through parenthesized_expression wrappers - Python
// and JS both require an invoked function literal to be parenthesized
// (e.g. `(lambda: ...)()`, `(function() {...})()`), so the call's function
// field is the parenthesized_expression, not the literal itself.
func unwrapParens(node *sitter.Node) *sitter.Node {
	for node != nil && node.Type() == "parenthesized_expression" && node.ChildCount() > 0 {
		var next *sitter.Node
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c.Type() != "(" && c.Type() != ")" {
				next = c
				break
			}
		}
		if next == nil {
			break
		}
		node = next
	}
	return node
}

func ExtractGoCallGraph(root *sitter.Node, source []byte) []CallEdge{

	var edges []CallEdge

	var walk func(node *sitter.Node, caller string)

	walk = func(node *sitter.Node, caller string) {

		if node == nil {
			return
		}

		nextCaller := caller

		//Track current function
		if node.Type() == "function_declaration" || node.Type() == "method_declaration" {

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {
				nextCaller = nameNode.Content(source)
			}
		} else if node.Type() == "func_literal" {
			// Anonymous function expression (e.g. `go func() { ... }()`) -
			// give it a synthetic name so calls inside it aren't
			// misattributed to the enclosing named function.
			nextCaller = anonymousFuncName(node)
		}

		//Detect function calls
		if node.Type() == "call_expression" {

			functionNode := node.ChildByFieldName("function")

			if functionNode != nil && caller != "" {

				var callee string
				if functionNode.Type() == "func_literal" {
					callee = anonymousFuncName(functionNode)
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

