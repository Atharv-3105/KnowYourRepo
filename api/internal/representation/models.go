package representation

type Symbol struct {
	ID     string 
	Name   string 
	Kind   string 
	Language string 
	FilePath   string 
	StartLine  int 
	EndLine    int 
}

type CallEdge struct {
	Caller   string 
	Callee   string 
}

type File  struct {
	Path    string 
	Language  string 
	Symbols   []Symbol
	Calls     []CallEdge
}

type Repository  struct {
	Name    string 
	Files   []File 
}

