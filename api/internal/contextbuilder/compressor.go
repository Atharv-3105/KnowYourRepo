package contextbuilder

//Function to Truncate the Size of the Document which is passed as context to the LLM
func TruncateDocument(doc string, maxChars int) string {

	if len(doc) <= maxChars {
		return doc 
	}

	return doc[:maxChars] + "\n..(truncated)"

}

func DeduplicateCalls(calls []string) []string {

	seen := make(map[string]struct{})

	var out []string 

	for _, c := range calls{

		if _, exists := seen[c]; exists {
			continue
		}

		seen[c] = struct{}{}

		out = append(out, c)
	}

	return out 
}

//Function to limit the call graph edges sent as context to the LLM
func LimitCalls(calls []string, maxCalls int) []string{

	if len(calls) <= maxCalls {
		return calls 
	}

	return calls[:maxCalls]
}