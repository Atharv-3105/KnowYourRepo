// This file will return entrypoints of functions by combining SymbolNames, FileNames, DirectoryLayout so that it becomes Language-Agnostic
package architecture

import (
	"path/filepath"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

func DetectEntrypoints(files []store.ArchitectureFile, symbols []store.ArchitectureSymbol) []EntryPoint {

	fileMap := make(map[int64]store.ArchitectureFile)

	for _, f := range files {
		fileMap[f.ID] = f
	}

	var entrypoints []EntryPoint

	for _, symbol := range symbols {

		file, ok := fileMap[symbol.FileID]
		if !ok {
			continue 
		}

		if !isEntrypoint(symbol, file) {
			continue 
		}

		entrypoints = append(entrypoints, buildEntrypoint(symbol, file))
	}

	return entrypoints
}

func isEntrypoint(symbol store.ArchitectureSymbol, file store.ArchitectureFile) bool {

	filename := strings.ToLower(filepath.Base(file.Path))
	dir := strings.ToLower(filepath.Dir(file.Path))

	switch strings.ToLower(file.Langauge) {
	case "go":

		if symbol.Name == "main"{
			return true
		}

		if strings.Contains(dir, "cmd"){
			return true
		}

		if filename == "main.go" {
			return true 
		}
	
	case "python":
		if filename == "main.py"{
			return true 
		}

		if symbol.Name == "__main__"{
			return true
		}

		if filename == "app.py"{
			return true 
		}

	case "javascript", "typescript":
		
		switch filename {

		case "index.js", "index.ts", "main.js", "main.ts", "server.js", "server.ts":
			return true 
		}
	}

	return false 
}

func buildEntrypoint(symbol store.ArchitectureSymbol, file store.ArchitectureFile) EntryPoint {

	entryType := "entrypoint"

	switch strings.ToLower(file.Langauge) {
	case "go":

		if symbol.Name == "main"{
			entryType = "main"
		}

	case "python":

		if symbol.Name == "__main__" {
			entryType = "__main__"
		}

	case "javascript", "typescript":

		entryType = "server"
	}


	return EntryPoint{
		Name:	symbol.Name,
		FilePath: file.Path,
		Language:  file.Langauge,
		Type:	   entryType,
	}
}