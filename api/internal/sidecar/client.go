package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

// InternalKeyHeader carries the shared secret the sidecar requires on every
// non-health request (see extraction-service/app/auth.py).
const InternalKeyHeader = "X-Internal-Key"

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			// Upper bound on any single sidecar call. Embedding is sent in
			// bounded batches (see api.RepoHandler.embedInBatches) of one
			// provider request each; this only bounds a call stuck behind
			// the provider's per-minute rate-limit retries.
			Timeout: 10 * time.Minute,
		},
	}
}

// WithAPIKey sets the shared secret sent on every request.
func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

func (c *Client) setHeaders(req *http.Request, ctx context.Context) {
	if id := config.FromContext(ctx); id != "" {
		req.Header.Set("X-Request-ID", id)
	}
	if c.apiKey != "" {
		req.Header.Set(InternalKeyHeader, c.apiKey)
	}
}

//Embed function of Client
func (c *Client) Embed(ctx context.Context, req EmbedBatchRequest) error {

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal batch embed request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/embed/batch",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return fmt.Errorf("failed to create batch embed request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	c.setHeaders(httpReq, ctx)

	//Perform a POST request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("batch embed request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return quotaErrorFrom(resp)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("batch embed request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *Client) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	c.setHeaders(httpReq, ctx)

	//Perform a POST request for Search
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, quotaErrorFrom(resp)
	}

	if resp.StatusCode != http.StatusOK {

		respBody, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("search request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var results []SearchResult

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return results, nil
}
