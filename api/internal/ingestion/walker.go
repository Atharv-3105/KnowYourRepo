package ingestion

import (
	"context"
	"log/slog"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	Path string  //Absolute File Path
	RelPath string //Relative File Path
	Language string 
}

type Walker struct {
	logger *slog.Logger
	ignoredDirs map[string]struct{}
}

func NewWalker(logger *slog.Logger) *Walker {
	return &Walker{
		logger:  logger,
		ignoredDirs: map[string]struct{}{
			".git": {},
			"node_modules": {},
			"dist":	{},
			"build": {},
			".next": {},
			"vendor": {},
			"venv": {},
			"__pycache__": {}, 
		},
	}
}

func (w *Walker) WalkRepo(ctx context.Context, root string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		//check for cancellation signal from the orchestrator
		select {
		case <-ctx.Done():
				return ctx.Err()
		default:
		}

		
		if err != nil{
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		if d.IsDir() {
			//skip hidden directories
			if d.Name() != filepath.Base(root) && strings.HasPrefix(d.Name(), ".") {
				if _, ok := w.ignoredDirs[d.Name()]; ok {
					return filepath.SkipDir
				}
			}
			//Skip ignored directories
			if _, ok := w.ignoredDirs[d.Name()]; ok {
				return filepath.SkipDir
			}
			return nil
		}

		lang := detectLanguage(path)
		if lang == "" {
			return nil //unsupported fileType
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		files = append(files, FileInfo{
			Path: path,
			RelPath: relPath,
			Language: lang,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	w.logger.Info("repository walk complete", "files_found", len(files))
	return files, nil
} 


//detectLanguage infers programming language 
func detectLanguage(path string) string {
	//get the lower-case value of the extension of filetype
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".cpp", ".c":
		return "cpp"
	case ".java":
		return "java"
	case ".html":
		return "html"
	case ".css":
		return "css"
	default:
		return ""
	}
}