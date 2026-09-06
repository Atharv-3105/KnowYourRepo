package store

import (
	"context"
	"fmt"
	"time"
)

type Repository struct {
	ID    	string
	RepoURL	string
}

func(s *Store) InsertRepository(ctx context.Context, repo Repository) error{
	query := `
	INSERT INTO repositories(id, repo_url)
	VALUES($1,$2)`

	_, err := s.db.ExecContext(ctx, query, repo.ID, repo.RepoURL)

	return err
}

// RepositorySummary is the shape needed by a "list repositories" view - it carries
// aggregate file/symbol counts alongside the base repository row, unlike Repository
// which is used by the plain CRUD lookups above (GetRepositoryByID/URL) that don't
// need stats.
type RepositorySummary struct {
	ID          string
	RepoURL     string
	CreatedAt   time.Time
	FileCount   int
	SymbolCount int
}

// ListRepositoriesWithStats returns every ingested repository with its file/symbol
// counts, newest first, in a single aggregate query rather than one query per repo
// (N+1) or reusing the full architecture.Analyzer (which loads every file/symbol/
// call-edge row into memory just to produce counts - too heavy for a list view).
func (s *Store) ListRepositoriesWithStats(ctx context.Context) ([]RepositorySummary, error) {

	query := `
	SELECT
		r.id,
		r.repo_url,
		r.created_at,
		COUNT(DISTINCT f.id) AS file_count,
		COUNT(DISTINCT sym.id) AS symbol_count
	FROM repositories r
	LEFT JOIN files f ON f.repo_id = r.id
	LEFT JOIN symbols sym ON sym.file_id = f.id
	GROUP BY r.id, r.repo_url, r.created_at
	ORDER BY r.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories with stats: %w", err)
	}
	defer rows.Close()

	var repos []RepositorySummary

	for rows.Next() {

		var repo RepositorySummary

		if err := rows.Scan(&repo.ID, &repo.RepoURL, &repo.CreatedAt, &repo.FileCount, &repo.SymbolCount); err != nil {
			return nil, fmt.Errorf("failed to scan repository summary: %w", err)
		}

		repos = append(repos, repo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate repository summaries: %w", err)
	}

	return repos, nil
}