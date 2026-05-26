package sidecar

import (
	"context"
	"testing"
)

func TestSearch(t *testing.T){

	client := NewClient(
		"http://localhost:8000",
	)

	results, err := client.Search(context.Background(),
								  SearchRequest{
										Query: "helllo function",
										Limit: 3,
								  },)

	if err != nil{
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("results: %+v", results)
}