package rag

import (
	"context"
	// "encoding/json"
	// "fmt"
	"log/slog"
	"github.com/atharva-3105/KnowYourRepo/internal/contextbuilder"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
)

type Service struct {
	builder *contextbuilder.Builder
	sidecar *sidecar.Client
	logger  *slog.Logger 
}

func NewService(builder *contextbuilder.Builder, sidecar *sidecar.Client, logger *slog.Logger) *Service {

	return &Service{
		builder: builder,
		sidecar: sidecar,
		logger: logger,
	}
}

func (s *Service) AnswerQuestion(ctx context.Context, query string, history string, results []retrieval.RetrievalResult) (string, error) {

	//Build the LLM Context
	buildContext := s.builder.Build(query, results)

	// contextJSON, err := json.MarshalIndent(buildContext, "", " ")

	prompt := buildPrompt(history, buildContext, query)

	s.logger.Info("rag_prompt_built", "history_chars:", len(history), "context_entries:", len(buildContext.Entries))

	// if err != nil {
	// 	return "", err
	// }

	s.logger.Info("context_for_LLM", "payload", prompt)

	s.logger.Info("retrieval_query", "query", query)

	// fmt.Println("=============CONTEXT SENT TO LLM=======")
	// fmt.Println(string(contextJSON))
	// fmt.Println("=======================================")

	//Send the context to LLM
	response, err := s.sidecar.Chat(ctx, sidecar.ChatRequest{
		Context: prompt,
		Question: query,
	})

	if err != nil {
		return "", err
	}

	return response.Answer, nil
}