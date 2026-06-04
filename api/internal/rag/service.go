package rag

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atharva-3105/KnowYourRepo/internal/contextbuilder"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
)

type Service struct {
	builder *contextbuilder.Builder
	sidecar *sidecar.Client
}

func NewService(builder *contextbuilder.Builder, sidecar *sidecar.Client) *Service {

	return &Service{
		builder: builder,
		sidecar: sidecar,
	}
}

func (s *Service) AnswerQuestion(ctx context.Context, query string, results []retrieval.RetrievalResult) (string, error) {

	//Build the LLM Context
	buildContext := s.builder.Build(query, results)

	contextJSON, err := json.MarshalIndent(buildContext, "", " ")

	if err != nil {
		return "", err
	}

	fmt.Println("=============CONTEXT SENT TO LLM=======")
	fmt.Println(string(contextJSON))
	fmt.Println("=======================================")

	//Send the context to LLM
	response, err := s.sidecar.Chat(ctx, sidecar.ChatRequest{
		Context: string(contextJSON),
		Question: query,
	})

	if err != nil {
		return "", err
	}

	return response.Answer, nil
}