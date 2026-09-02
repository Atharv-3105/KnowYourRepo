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

type Client struct {
	baseURL   string 
	httpClient      *http.Client
}

func NewClient(baseURL string) *Client{

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

func setRequestIDHeader(req *http.Request, ctx context.Context) {
	if id :=  config.FromContext(ctx);id != "" {
		req.Header.Set("X-Request-ID", id)
	}
}


//Embed function of Client
func (c *Client) Embed(ctx context.Context,req EmbedBatchRequest,) error{

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
	setRequestIDHeader(httpReq, ctx)

	//Perform a POST request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("batch embed request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK{
		respBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("batch embed request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}


func (c *Client) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {

	body, err := json.Marshal(req)
	if err != nil{
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewBuffer(body))
	if err != nil{
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	setRequestIDHeader(httpReq, ctx)

	//Perform a POST request for Search
	resp, err := c.httpClient.Do(httpReq)
	if err != nil{
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK{

		respBody, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("search request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var results []SearchResult

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil{
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return results, nil
}
