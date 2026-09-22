package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OverviewEntrypoint struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Language string `json:"language"`
}

type OverviewConcept struct {
	Term        string `json:"term"`
	Explanation string `json:"explanation"`
	// Symbol is optional - the LLM's own claimed name for a real symbol
	// this concept relates to, if any. Never trusted as a location by
	// itself; the caller (architecture.Service.GenerateOverview) resolves
	// it against the real symbol index before using it as a citation.
	Symbol string `json:"symbol,omitempty"`
}

type GenerateOverviewRequest struct {
	ReadmeText            string               `json:"readme_text"`
	Entrypoints           []OverviewEntrypoint `json:"entrypoints"`
	DirectoryStructure    []string             `json:"directory_structure"`
	RepresentativeSymbols []string             `json:"representative_symbols"`
}

type GenerateOverviewResponse struct {
	NarrativeSummary string            `json:"narrative_summary"`
	Concepts         []OverviewConcept `json:"concepts"`
}

func (c *Client) GenerateOverview(ctx context.Context, req GenerateOverviewRequest) (*GenerateOverviewResponse, error) {

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal generate-overview request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/generate-overview", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create generate-overview request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	setRequestIDHeader(httpReq, ctx)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("generate-overview request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("generate-overview request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result GenerateOverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode generate-overview response: %w", err)
	}

	return &result, nil
}
