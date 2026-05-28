package representation


func NewFile(path string, language string) File {

	return File {
		Path:	path,
		Language: language,
		Symbols:  []Symbol{},
		Calls: 	  []CallEdge{},
	}
}

