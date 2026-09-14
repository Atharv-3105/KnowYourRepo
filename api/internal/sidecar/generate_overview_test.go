package sidecar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateOverview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/generate-overview" {
			t.Errorf("expected path /generate-overview, got %s", r.URL.Path)
		}

		var req GenerateOverviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.ReadmeText != "# Test" {
			t.Errorf("unexpected readme_text: %q", req.ReadmeText)
		}

		resp := GenerateOverviewResponse{
			NarrativeSummary: "This is a test project.",
			Concepts: []OverviewConcept{
				{Term: "Worker pool", Explanation: "Processes jobs concurrently."},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	resp, err := client.GenerateOverview(context.Background(), GenerateOverviewRequest{
		ReadmeText:            "# Test",
		Entrypoints:           []OverviewEntrypoint{{Name: "main", FilePath: "main.go", Language: "go"}},
		DirectoryStructure:    []string{"internal", "cmd"},
		RepresentativeSymbols: []string{"main (function)"},
	})
	if err != nil {
		t.Fatalf("GenerateOverview failed: %v", err)
	}
	if resp.NarrativeSummary != "This is a test project." {
		t.Errorf("unexpected narrative_summary: %q", resp.NarrativeSummary)
	}
	if len(resp.Concepts) != 1 || resp.Concepts[0].Term != "Worker pool" {
		t.Errorf("unexpected concepts: %+v", resp.Concepts)
	}
}

func TestGenerateOverview_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("provider exhausted"))
	}))
	defer server.Close()

	client := NewClient(server.URL)

	_, err := client.GenerateOverview(context.Background(), GenerateOverviewRequest{})
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}
}
