package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// FileRecord is a previously-ingested file's identity, used to diff
// against a fresh filesystem walk during incremental re-indexing.
type FileRecord struct {
	ID   int64
	Hash string
	// EmbeddedModel is the embedding model this file's chunks were embedded
	// with, or "" if they haven't been (yet).
	EmbeddedModel string
}

func (s *Store) GetRepositoryByURL(ctx context.Context, repoURL string) (*Repository, error) {

	// Case-insensitive: GitHub treats owner/repo case-insensitively, so
	// "Owner/Repo" and "owner/repo" are the same repository and must not
	// become two separate ingestions.
	query := `SELECT id, repo_url FROM repositories WHERE LOWER(repo_url) = LOWER($1) ORDER BY created_at LIMIT 1`

	var repo Repository

	err := s.db.QueryRowContext(ctx, query, repoURL).Scan(&repo.ID, &repo.RepoURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get repository by url: %w", err)
	}

	return &repo, nil
}

// TouchRepoCheck records that a repo was just checked/synced against its
// remote, storing the commit SHA seen at that point. Called both when the
// scheduler finds no change (SHA unchanged) and after a real sync
// completes (SHA updated) - same bookkeeping either way.
func (s *Store) TouchRepoCheck(ctx context.Context, repoID, commitSHA string) error {

	query := `
	UPDATE repositories
	SET last_commit_sha = $1, last_checked_at = now()
	WHERE id = $2
	`

	_, err := s.db.ExecContext(ctx, query, commitSHA, repoID)
	if err != nil {
		return fmt.Errorf("failed to update repo sync state: %w", err)
	}

	return nil
}

func (s *Store) GetLastCommitSHA(ctx context.Context, repoID string) (string, error) {

	query := `SELECT COALESCE(last_commit_sha, '') FROM repositories WHERE id = $1`

	var sha string

	err := s.db.QueryRowContext(ctx, query, repoID).Scan(&sha)
	if err != nil {
		return "", fmt.Errorf("failed to get last commit sha: %w", err)
	}

	return sha, nil
}

func (s *Store) GetFileHashesByRepo(ctx context.Context, repoID string) (map[string]FileRecord, error) {

	query := `SELECT id, path, hash, embedded_model FROM files WHERE repo_id = $1`

	rows, err := s.db.QueryContext(ctx, query, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file hashes: %w", err)
	}
	defer rows.Close()

	result := make(map[string]FileRecord)

	for rows.Next() {

		var id int64
		var path string
		var hash, embeddedModel sql.NullString

		if err := rows.Scan(&id, &path, &hash, &embeddedModel); err != nil {
			return nil, err
		}

		result[path] = FileRecord{ID: id, Hash: hash.String, EmbeddedModel: embeddedModel.String}
	}

	return result, rows.Err()
}

// DeleteFileByPath removes a file (and, via ON DELETE CASCADE, its
// symbols) plus its call_edges - call_edges has no FK to files since it's
// keyed by caller_file_path directly, so it needs its own delete.
func (s *Store) DeleteFileByPath(ctx context.Context, repoID, path string) error {

	_, err := s.db.ExecContext(ctx, `DELETE FROM call_edges WHERE repo_id = $1 AND caller_file_path = $2`, repoID, path)
	if err != nil {
		return fmt.Errorf("failed to delete call edges for file: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM files WHERE repo_id = $1 AND path = $2`, repoID, path)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func(s *Store) GetRepositoryByID(ctx context.Context, repoID string)(*Repository, error){

	query := `SELECT id, repo_url FROM repositories WHERE id = $1`

	var repo Repository

	err := s.db.QueryRowContext(ctx, query, repoID).Scan(&repo.ID, &repo.RepoURL)
	if err != nil{
		if err == sql.ErrNoRows{
			return nil, nil 
		}
		return nil, fmt.Errorf("failed to get repository by id: %w", err)
	}

	return &repo, nil 
}

//DeleteSymbolsAndCallEdges function clears a file's symbols and call_edges without deleting the file row iteself,
//used when a file changed and is about to be re-parsed
func(s *Store) DeleteSymbolsAndCallEdges(ctx context.Context, repoID string, fileID int64, path string) error{

	_, err := s.db.ExecContext(ctx, `DELETE FROM symbols WHERE file_id = $1`, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete stale symbols: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM call_edges WHERE repo_id = $1 AND caller_file_path = $2`, repoID, path)
	if err != nil {
		return fmt.Errorf("failed to delete stale call edges: %w", err)
	}

	return nil 
}

func(s *Store) GetAllRepositories(ctx context.Context) ([]Repository, error){

	query := `SELECT id, repo_url FROM repositories`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer rows.Close()

	var repos []Repository

	for rows.Next() {

		var repo Repository

		if err := rows.Scan(&repo.ID, &repo.RepoURL); err != nil {
			return nil, err 
		}

		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// GetLastCheckedAt returns when the repo was last checked against its
// remote. ok is false if it has never been checked.
func (s *Store) GetLastCheckedAt(ctx context.Context, repoID string) (t time.Time, ok bool, err error) {
	var checked sql.NullTime
	err = s.db.QueryRowContext(ctx, `SELECT last_checked_at FROM repositories WHERE id = $1`, repoID).Scan(&checked)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("failed to get last checked time: %w", err)
	}
	return checked.Time, checked.Valid, nil
}

// MarkFilesEmbedded records that every chunk of these files is embedded with
// model.
func (s *Store) MarkFilesEmbedded(ctx context.Context, repoID string, paths []string, model string) error {
	if len(paths) == 0 {
		return nil
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE files SET embedded_model = $3 WHERE repo_id = $1 AND path = ANY($2)`,
		repoID, paths, model,
	)
	if err != nil {
		return fmt.Errorf("failed to mark files embedded: %w", err)
	}
	return nil
}

// CountFilesPendingEmbedding counts a repo's files not (fully) embedded with
// model - left over from an exhausted quota, the per-run chunk budget, or a
// model change.
func (s *Store) CountFilesPendingEmbedding(ctx context.Context, repoID, model string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM files WHERE repo_id = $1 AND embedded_model IS DISTINCT FROM $2`,
		repoID, model,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("failed to count files pending embedding: %w", err)
	}
	return n, nil
}
