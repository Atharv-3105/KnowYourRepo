package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ErrEmbeddingQuotaExhausted is returned when the sidecar reports (HTTP 429)
// that the embedding provider's quota is used up. Retrying before
// QuotaError.RetryAfter only fails again.
var ErrEmbeddingQuotaExhausted = errors.New("embedding quota exhausted")

type QuotaError struct {
	RetryAfter time.Duration
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("%v (retry after %s)", ErrEmbeddingQuotaExhausted, e.RetryAfter)
}

func (e *QuotaError) Unwrap() error { return ErrEmbeddingQuotaExhausted }

func quotaErrorFrom(resp *http.Response) *QuotaError {
	seconds, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
	return &QuotaError{RetryAfter: time.Duration(seconds) * time.Second}
}

// EmbedInfo describes the sidecar's current embedding model and quota.
type EmbedInfo struct {
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	Dimension         int    `json:"dimension"`
	DailyLimit        int    `json:"daily_limit"`
	DailyUsed         int    `json:"daily_used"`
	Exhausted         bool   `json:"exhausted"`
	RetryAfterSeconds int    `json:"retry_after_seconds"`
}

// EmbedInfo fetches the embedding model the sidecar is currently using. It
// makes no provider call, so it costs no quota.
func (c *Client) EmbedInfo(ctx context.Context) (*EmbedInfo, error) {

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/embed/info", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create embed info request: %w", err)
	}
	c.setHeaders(httpReq, ctx)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embed info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embed info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var info EmbedInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode embed info: %w", err)
	}
	if info.Model == "" {
		return nil, errors.New("sidecar reported no embedding model")
	}
	return &info, nil
}
