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
	VALUES  ($1, $2, $3)
	ON CONFLICT(repo_id, path) DO UPDATE SET language = EXCLUDED.language
	RETURNING id
	`

	//By switching to Postgre we don't need to do the 2-step "insert then SELECT the existing ID on conflict"
	//RETURNING ensures data is returned even when ON CONFLICT DO UPDATE
	var id int64 

	err := s.db.QueryRowContext(ctx, query, repoID, path, language).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert file: %w", err)
	}

	return id, nil 
}