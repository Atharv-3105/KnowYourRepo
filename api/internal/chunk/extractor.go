package chunk   

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)


func ExtractFunctionChunks(root *sitter.Node, source []byte, language string, filePath string) []Chunk{

	var chunks []Chunk

	var walk  func(node *sitter.Node)

	walk = func(node *sitter.Node) {

		if node == nil {
			return
		}

		var isFunction bool 

		switch language {

		case "go":
			isFunction = node.Type() == "function_declaration" || node.Type() == "method_declaration"
		case "python":
			isFunction = node.Type() == "function_definition"
		case "javascript", "typescript":
			isFunction = node.Type() == "function_declaration"
		}

		if isFunction {

			nameNode := node.ChildByFieldName("name")

			if nameNode != nil {

				name := nameNode.Content(source)

				content := node.Content(source)

				chunks = append(chunks, Chunk{
						ID:  fmt.Sprintf("%s:%d", filePath, node.StartPoint().Row,),
						FilePath: filePath,
						Language: language,
						SymbolName: name,
						Content: content,
						StartLine: int(node.StartPoint().Row) + 1,
						EndLine:  int(node.EndPoint().Row) + 1,
				})
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)

	return chunks
}