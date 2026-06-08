package rag

import (
	"encoding/json"
	"fmt"

	"github.com/atharva-3105/KnowYourRepo/internal/contextbuilder"
)

func buildPrompt(history string, ctxPkg contextbuilder.ContextPackage, question string) string {

	contextJSON, _ := json.MarshalIndent(ctxPkg, ""," ")

	return fmt.Sprintf(`
	You are KnowYourRepo.

	Use the repository context to answer questions.

	Conversation History:
	%s

	Repository Context:
	%s

	Current Question:
	%s

	Answer based ONLY on repository context.
	`, history, string(contextJSON), question)
}