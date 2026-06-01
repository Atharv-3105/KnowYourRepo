//Smart Chunking Strategy
//Since for large functions embedding will become noisy, it will have huge number of tokens
//So we will do Function->Chunk Splitter->Multiple Chunks->Embedding
package chunk

const (
	MaxChunkChars = 1500
	OverLapChars = 200
)

func SplitChunk(c Chunk) []Chunk {

	if len(c.Content) <= MaxChunkChars {
		return []Chunk{c}
	}

	var chunks []Chunk

	start := 0

	for start < len(c.Content) {

		end := start + MaxChunkChars

		if end > len(c.Content) {
			end = len(c.Content)
		}

		chunkContent := c.Content[start:end]

		chunks = append(chunks, Chunk{
			ID:		c.ID,
			FilePath: c.FilePath,
			Language: c.Language,
			SymbolName: c.SymbolName,
			Content: 	chunkContent,
			StartLine:   c.StartLine,
			EndLine:     c.EndLine,
		})

		if end == len(c.Content){
			break	
		}

		start = end - OverLapChars
	}

	return chunks
}