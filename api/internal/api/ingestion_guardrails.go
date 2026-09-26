package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
	"github.com/atharva-3105/KnowYourRepo/internal/metrics"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
)

// embedInBatches sends embed items to the sidecar in bounded batches rather
// than as one request covering the whole repository, and returns how many
// items (a prefix of items) were embedded.
//
// Each batch is one embedding-provider request (EmbedBatchItems matches the
// sidecar's provider batch size), and free embedding models are limited per
// request per day. Bounded batches persist progress as they go - a batch
// that succeeded stays embedded even if a later one fails, and the caller
// marks those files done so the next sync resumes after them - and stop at
// the first failure rather than hammering an exhausted quota with the rest.
func (h *RepoHandler) embedInBatches(ctx context.Context, repoID string, items []sidecar.EmbedBatchItem) (int, error) {
	size := h.limits.EmbedBatchItems
	if size <= 0 {
		size = 128
	}

	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}

		if err := h.sidecar.Embed(ctx, sidecar.EmbedBatchRequest{Items: items[start:end]}); err != nil {
			metrics.EmbedChunksTotal.WithLabelValues("failed").Add(float64(len(items) - start))
			return start, fmt.Errorf("embedding batch %d-%d of %d failed: %w", start, end, len(items), err)
		}

		metrics.EmbedChunksTotal.WithLabelValues("embedded").Add(float64(end - start))
		h.logger.Info("embed_batch_complete", "repo_id", repoID, "done", end, "total", len(items))
	}

	return len(items), nil
}

// fileChunkSpan is the range of one file's chunks within the ingestion
// run's flat list of embed items.
type fileChunkSpan struct {
	path       string
	start, end int
}

// fullyEmbeddedFiles returns the files all of whose chunks fall within the
// first embedded items. A file with no chunks has nothing to embed and is
// always done.
func fullyEmbeddedFiles(spans []fileChunkSpan, embedded int) []string {
	var paths []string
	for _, s := range spans {
		if s.end <= embedded || s.start == s.end {
			paths = append(paths, s.path)
		}
	}
	return paths
}

// shouldGenerateOverview skips the LLM overview call on a sync that changed
// nothing (a freshness check that found the remote unchanged still runs the
// pipeline, which then skips every file by hash). The existing overview is
// still accurate, and regenerating it would spend an LLM call - against a
// free-tier daily quota - to produce the same text.
func (h *RepoHandler) shouldGenerateOverview(ctx context.Context, repoID string, processed, deleted int) bool {
	if processed > 0 || deleted > 0 {
		return true
	}

	overview, err := h.store.GetOverview(ctx, repoID)
	if err != nil || overview == nil {
		return true
	}
	return overview.NarrativeSummary == ""
}

// userFacingJobError turns an ingestion failure into the message shown on
// the progress page. Internal errors can carry file paths, SQL, or
// upstream-provider responses; those stay in the logs.
func userFacingJobError(err error) string {
	var tooLarge *ingestion.ErrRepoTooLarge

	switch {
	case errors.As(err, &tooLarge):
		return fmt.Sprintf("This repository is %dMB, above this deployment's %dMB limit.", tooLarge.SizeMB, tooLarge.LimitMB)
	case errors.Is(err, ingestion.ErrRepoNotFound):
		return "Repository not found, or it isn't public."
	case errors.Is(err, context.DeadlineExceeded):
		return "Ingestion timed out. Please try again."
	case strings.HasPrefix(err.Error(), "clone failed"), strings.HasPrefix(err.Error(), "sync failed"):
		return "Could not fetch the repository from GitHub. It may be private, empty, or temporarily unavailable."
	default:
		return "Ingestion failed due to an internal error. Please try again."
	}
}
