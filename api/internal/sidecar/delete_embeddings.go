package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type DeleteEmbeddingsRequest struct {
	RepoID   string `json:"repo_id"`
	FilePath string `json:"file_path"`
}

type DeleteEmbeddingsResponse struct {
	Success bool `json:"success"`
	Deleted int  `json:"deleted"`
}

func (c *Client) DeleteEmbeddings(ctx context.Context, req DeleteEmbeddingsRequest) error {

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal delete embeddings request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx,
		http.MethodDelete,
		c.baseURL+"/embed",
		bytes.NewBuffer(body))

	if err != nil {
		return fmt.Errorf("failed to create delete embeddings request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	setRequestIDHeader(httpReq, ctx)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("delete embeddings request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete embeddings request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}