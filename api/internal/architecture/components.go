package architecture

import (
	"path/filepath"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

var architecturalSuffixes = []string{
	"Manager","Service","Repository","Repo","Controller","Handler","Client","Server","Hub","Store","Parser","Extractor","Builder","Walker","Factory","Provider","Agent","Engine","Scheduler","Coordinator","Cache",
}

func DetectComponents(files []store.ArchitectureFile, symbols []store.ArchitectureSymbol) []Component{

	//Use a map to store (file_id to file) relation
	fileMap := make(map[int64]store.ArchitectureFile)

	for _, f := range files {
		fileMap[f.ID] = f
	}

	var components []Component

	for _, sym := range symbols {

		file, ok := fileMap[sym.FileID]

		if !ok {
			continue 
		}

		if !isArchitecturalComponent(sym, file){
			continue 
		}

		components = append(components, buildComponent(sym, file))
	}

	return components
}

//we check for suffixes or type which can be architectural component
func isArchitecturalComponent(symbol store.ArchitectureSymbol, file store.ArchitectureFile) bool {

	switch strings.ToLower(symbol.Type){
	case "struct":
		return true
	case "interface":
		return true 
	case "class":
		return true
	}

	for _, suffix := range architecturalSuffixes  {

		if strings.HasSuffix(symbol.Name, suffix) {
			return true 
		}
	}

	//we check filename heuristics so if Tree-Sitter missed it, we can still get architecturalComponent
	filename := strings.ToLower(filepath.Base(file.Path))

	if strings.Contains(filename, "handler") || strings.Contains(filename, "controller") || strings.Contains(filename, "service") ||
	   strings.Contains(filename, "repository") || strings.Contains(filename, "manager") || strings.Contains(filename, "agent") {
		return true 
	   }

	return false 
}

func buildComponent(symbol store.ArchitectureSymbol, file store.ArchitectureFile) Component {
	return Component{
		Name: 	symbol.Name,
		Type:	symbol.Type,
		FilePath: file.Path,
		Language:	file.Langauge,
		StartLine:  symbol.StartLine,
		EndLine: 	symbol.EndLine,
	}
}

