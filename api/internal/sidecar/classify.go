package sidecar 

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type ClassifyRequest struct {
	Question string `json:"question"`
	History string  `json:"history"`
}

type ClassifyResponse struct {
	Tools []string `json:"tools"`
}

//This error is returned when the sidecar's(python service) /classify endpoint reports every LLM provider is currently unavailable(HTTP 503)
//Callers (the agent's hybrid planner) should treat this as "fall back to the deterministic planner" rather than a hard failure
var ErrClassificationUnavailable = errors.New("sidecar: classification unavailable")


//Method for hitting /classify endpoint and parsing the response
func (c *Client) Classify(ctx context.Context, req ClassifyRequest) (*ClassifyResponse, error) {

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal classify request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx,
			http.MethodPost,
			c.baseURL + "/classify",
			bytes.NewBuffer(body),
	)	

	if err != nil {
		return nil, fmt.Errorf("failed to create classify request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	setRequestIDHeader(httpReq, ctx)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil{
		return nil, fmt.Errorf("classify request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, ErrClassificationUnavailable
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("classify request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ClassifyResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil{
		return nil, fmt.Errorf("failed to decode classify response: %w", err)
	}

	return &result, nil 
}