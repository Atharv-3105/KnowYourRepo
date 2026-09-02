package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ChatRequest struct {
	Context  string `json:"context"`
	Question string `json:"question"`
}

type ChatResponse struct {
	Answer string `json:"answer"`
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to marshal chat request: %w",
			err,
		)
	}

	httpReq, err := http.NewRequestWithContext(ctx, 
											   http.MethodPost,
											   c.baseURL + "/chat",
												bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create chat request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	setRequestIDHeader(httpReq, ctx)

	resp, err := c.httpClient.Do(httpReq)

	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK{

		respBody, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"chat request failed with status %d: %s", resp.StatusCode, string(respBody),
		)
	}

	var result ChatResponse

	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode chat response: %w",
			err,
		)
	}

	return &result, nil
}