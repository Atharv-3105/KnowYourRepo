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


func(s *Store) InsertFile(ctx context.Context, repoID, path, language, hash string) (int64, error) {
	query := `
	INSERT INTO files (repo_id, path, language, hash)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (repo_id, path) DO UPDATE SET language = EXCLUDED.language, hash = EXCLUDED.hash
	RETURNING id
	`

	var id int64

	err := s.db.QueryRowContext(ctx, query, repoID, path, language, hash).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert file: %w", err)
	}

	return id, nil
}