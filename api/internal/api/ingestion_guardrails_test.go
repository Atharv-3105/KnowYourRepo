package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/atharva-3105/KnowYourRepo/internal/config"
	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
)

func TestUserFacingJobError_NeverLeaksInternals(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&ingestion.ErrRepoTooLarge{SizeMB: 400, LimitMB: 150}, "400MB"},
		{fmt.Errorf("wrapped: %w", ingestion.ErrRepoNotFound), "not found"},
		{errors.New("clone failed: authentication required"), "Could not fetch"},
		{context.DeadlineExceeded, "timed out"},
		{errors.New(`pq: relation "files" does not exist at ..\data\repos\repo_1\x.go`), "internal error"},
	}

	for _, c := range cases {
		got := userFacingJobError(c.err)
		if !strings.Contains(got, c.want) {
			t.Errorf("userFacingJobError(%v) = %q, want it to contain %q", c.err, got, c.want)
		}
		if strings.Contains(got, "data") || strings.Contains(got, "relation") {
			t.Errorf("userFacingJobError leaked internal detail: %q", got)
		}
	}
}

func handlerWithSidecar(t *testing.T, batchSize int, fail func(call int) bool) (*RepoHandler, *atomic.Int32, *[]int) {
	t.Helper()
	var calls atomic.Int32
	var sizes []int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		var req sidecar.EmbedBatchRequest
		json.NewDecoder(r.Body).Decode(&req)
		sizes = append(sizes, len(req.Items))
		if fail != nil && fail(n) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	h := &RepoHandler{
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		sidecar: sidecar.NewClient(server.URL),
		limits:  config.LimitsConfig{EmbedBatchItems: batchSize},
	}
	return h, &calls, &sizes
}

func makeItems(n int) []sidecar.EmbedBatchItem {
	items := make([]sidecar.EmbedBatchItem, n)
	for i := range items {
		items[i] = sidecar.EmbedBatchItem{ID: fmt.Sprint(i), Text: "x"}
	}
	return items
}

// A whole repository used to go to the sidecar as one request - a long
// call that lost everything if it failed at the end. It's now sent in
// bounded batches, one provider request each.
func TestEmbedInBatches_SplitsIntoBoundedRequests(t *testing.T) {
	h, calls, sizes := handlerWithSidecar(t, 10, nil)

	embedded, err := h.embedInBatches(context.Background(), "repo", makeItems(25))
	if err != nil {
		t.Fatalf("embedInBatches failed: %v", err)
	}
	if embedded != 25 {
		t.Errorf("expected 25 embedded, got %d", embedded)
	}

	if calls.Load() != 3 {
		t.Errorf("expected 3 requests for 25 items in batches of 10, got %d", calls.Load())
	}
	if len(*sizes) != 3 || (*sizes)[0] != 10 || (*sizes)[2] != 5 {
		t.Errorf("unexpected batch sizes %v", *sizes)
	}
}

// Once the provider quota is exhausted, sending the remaining batches just
// burns more of it - stop at the first failure.
func TestEmbedInBatches_StopsAtFirstFailure(t *testing.T) {
	h, calls, _ := handlerWithSidecar(t, 10, func(call int) bool { return call == 2 })

	embedded, err := h.embedInBatches(context.Background(), "repo", makeItems(50))
	if err == nil {
		t.Fatal("expected an error when a batch fails")
	}
	if embedded != 10 {
		t.Errorf("expected only the first batch (10) to count as embedded, got %d", embedded)
	}
	if !errors.Is(err, sidecar.ErrEmbeddingQuotaExhausted) {
		t.Errorf("expected a 429 from the sidecar to surface as ErrEmbeddingQuotaExhausted, got %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected embedding to stop after the failed 2nd batch, got %d calls", calls.Load())
	}
}

func TestFullyEmbeddedFiles(t *testing.T) {
	spans := []fileChunkSpan{
		{path: "a.go", start: 0, end: 3},
		{path: "empty.go", start: 3, end: 3},
		{path: "b.go", start: 3, end: 7},  // straddles the failure point
		{path: "c.go", start: 7, end: 9},  // never reached
		{path: "late-empty.go", start: 9, end: 9},
	}

	got := fullyEmbeddedFiles(spans, 5)
	want := []string{"a.go", "empty.go", "late-empty.go"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("fullyEmbeddedFiles = %v, want %v", got, want)
	}
}
