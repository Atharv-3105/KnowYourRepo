package contextbuilder

type ContextPackage struct {
	Query 		string 		`json:"query"`
	Entries     []ContextNode  `json:"entries"`
}

type ContextNode   struct {
	Symbol      string    `json:"symbol"`
	Document    string    `json:"document"`
	Calls      []string    `json:"calls"`
}

