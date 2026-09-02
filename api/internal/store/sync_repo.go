package store

import (
	"context"
	"database/sql"
	"fmt"
)

// FileRecord is a previously-ingested file's identity, used to diff
// against a fresh filesystem walk during incremental re-indexing.
type FileRecord struct {
	ID   int64
	Hash string
}

func (s *Store) GetRepositoryByURL(ctx context.Context, repoURL string) (*Repository, error) {

	query := `SELECT id, repo_url FROM repositories WHERE repo_url = $1`

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

	query := `SELECT id, path, hash FROM files WHERE repo_id = $1`

	rows, err := s.db.QueryContext(ctx, query, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file hashes: %w", err)
	}
	defer rows.Close()

	result := make(map[string]FileRecord)

	for rows.Next() {

		var id int64
		var path string
		var hash sql.NullString

		if err := rows.Scan(&id, &path, &hash); err != nil {
			return nil, err
		}

		result[path] = FileRecord{ID: id, Hash: hash.String}
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