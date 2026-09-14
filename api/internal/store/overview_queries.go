package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Concept struct {
	Term        string `json:"term"`
	Explanation string `json:"explanation"`
}

type Overview struct {
	RepoID           string
	NarrativeSummary string
	Concepts         []Concept
	GeneratedAt      time.Time
}

// SaveOverview upserts the generated overview for a repo - called once per
// ingestion/sync from architecture.Service.GenerateOverview. A concepts
// slice of nil or zero length is stored as a JSON empty array, not SQL NULL,
// so GetOverview's caller never has to distinguish "no concepts" from
// "generation hasn't run" by inspecting this column alone.
func (s *Store) SaveOverview(ctx context.Context, repoID, narrativeSummary string, concepts []Concept) error {
	if concepts == nil {
		concepts = []Concept{}
	}

	conceptsJSON, err := json.Marshal(concepts)
	if err != nil {
		return fmt.Errorf("failed to marshal concepts: %w", err)
	}

	query := `
		INSERT INTO repo_overview (repo_id, narrative_summary, concepts, generated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (repo_id) DO UPDATE SET
			narrative_summary = EXCLUDED.narrative_summary,
			concepts = EXCLUDED.concepts,
			generated_at = EXCLUDED.generated_at
	`

	if _, err := s.db.ExecContext(ctx, query, repoID, narrativeSummary, conceptsJSON); err != nil {
		return fmt.Errorf("failed to save overview: %w", err)
	}

	return nil
}

// GetOverview returns (nil, nil) - not an error - when no overview row
// exists yet for repoID (generation hasn't run, or failed and left nothing
// to store). Callers (architecture.Analyzer) treat that as "no narrative
// summary/concepts available", not a request failure.
func (s *Store) GetOverview(ctx context.Context, repoID string) (*Overview, error) {
	query := `
		SELECT repo_id, narrative_summary, concepts, generated_at
		FROM repo_overview
		WHERE repo_id = $1
	`

	var o Overview
	var conceptsJSON []byte

	err := s.db.QueryRowContext(ctx, query, repoID).Scan(&o.RepoID, &o.NarrativeSummary, &conceptsJSON, &o.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get overview: %w", err)
	}

	if err := json.Unmarshal(conceptsJSON, &o.Concepts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal concepts: %w", err)
	}

	return &o, nil
}
