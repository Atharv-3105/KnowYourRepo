package sidecar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearch(t *testing.T){
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		results := []SearchResult{
			{
				ID:       "chunk_1",
				Document: "func test() {}",
				Metadata: map[string]interface{}{"symbol": "test"},
				Distance: 0.1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	results, err := client.Search(context.Background(),
								  SearchRequest{
										Query: "helllo function",
										Limit: 3,
								  },)

	if err != nil{
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 || results[0].ID != "chunk_1" {
		t.Errorf("unexpected results: %+v", results)
	}

	t.Logf("results: %+v", results)
}
// Every sidecar route except /health requires the shared secret - the
// client must attach it to every call once configured.
func TestClient_SendsInternalKeyHeader(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(InternalKeyHeader)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SearchResult{})
	}))
	defer server.Close()

	client := NewClient(server.URL).WithAPIKey("s3cret")
	if _, err := client.Search(context.Background(), SearchRequest{Query: "q", Limit: 1}); err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if gotKey != "s3cret" {
		t.Errorf("expected %s header %q, got %q", InternalKeyHeader, "s3cret", gotKey)
	}
}
