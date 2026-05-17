package ingestion

import(
	"context"
	"log/slog"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	tsjs   "github.com/smacker/go-tree-sitter/javascript"
	tspy   "github.com/smacker/go-tree-sitter/python"
	tsts   "github.com/smacker/go-tree-sitter/typescript/typescript"
	tstsx  "github.com/smacker/go-tree-sitter/typescript/tsx"
	tsgo   "github.com/smacker/go-tree-sitter/golang"
)

type ParseResult struct {
	FilePath  string 
	Language  string 
	Root      *sitter.Node
	Tree      *sitter.Tree  
	Source    []byte
}

type TreeSitterParser  struct{
	logger *slog.Logger
}

func NewParser(logger *slog.Logger) *TreeSitterParser {
	return &TreeSitterParser{
		logger: logger,
	}
}

func (p *TreeSitterParser) ParseFile(ctx context.Context, path string, language string) (*ParseResult, error) {
	p.logger.Debug("parsing file", "path",path, "lang", language)

	select {
	case <- ctx.Done():
		return nil, ctx.Err()
	default:
	}

	source , err := os.ReadFile(path)
	if err != nil {
		p.logger.Error("failed to read file", "path", path, "error", err)
		return nil, fmt.Errorf("failed to read file %s:%w", path, err)
	}

	lang, err := getLanguage(language)
	if err != nil {
		p.logger.Warn("unsupported language", "lang", language, "path", path)
		return nil, err
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil{
		p.logger.Error("tree-sitter parse error", "path", path, "error", err)
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if tree == nil {
		p.logger.Error("tree-sitter returned nil tree", "path", path)
		return nil, fmt.Errorf("tree-sitter failed to parse %s", path)
	}

	p.logger.Info("file parsed successfully","path", path, "size_bytes", len(source), "root_type", tree.RootNode().Type())

	return &ParseResult{
		FilePath: path,
		Language: language,
		Root: tree.RootNode(),
		Tree: tree,
		Source: source,
	}, nil
}

func getLanguage(lang string) (*sitter.Language, error) {
	switch lang{
	case "go":
		return tsgo.GetLanguage(), nil
	case "python":
		return tspy.GetLanguage(), nil
	case "javascript":
		return tsjs.GetLanguage(), nil
	case "typescript":
		return tsts.GetLanguage(), nil
	case "tsx":
		return tstsx.GetLanguage(), nil
	default:
		return nil, fmt.Errorf("unsupported tree-sitter language: %s", lang)
	}
}