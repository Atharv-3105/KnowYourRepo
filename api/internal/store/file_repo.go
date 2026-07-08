package store 


import (
	"fmt"
	"context"
)

type File struct {
	ID    int64
	Path  string 
	Language string
}


func(s *Store) InsertFile(ctx context.Context, repoID, path, language string) (int64, error) {
	query := `
	INSERT INTO files (repo_id, path, language)
	VALUES  (?, ?, ?)
	ON CONFLICT(repo_id, path) DO NOTHING
	`

	res, err := s.db.ExecContext(ctx, query, repoID, path, language)
	if err != nil {
		return 0, fmt.Errorf("failed to insert file: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected > 0 {
		id, _ := res.LastInsertId()
		return id, nil
	}

	var existingID int64
	err = s.db.QueryRowContext(ctx, "SELECT id FROM files WHERE repo_id = ? AND path = ?", repoID, path).Scan(&existingID)
	if err != nil {
		return 0, fmt.Errorf("failed to get existing file ID: %w", err)
	}
	return existingID, nil
}