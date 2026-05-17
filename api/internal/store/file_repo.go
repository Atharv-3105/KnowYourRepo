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


func(s *Store) InsertFile(ctx context.Context, path, language string) (int64, error) {
	query := `
	INSERT INTO files (path, language)
	VALUES  (?, ?)
	ON CONFLICT(path) DO NOTHING
	`

	res, err := s.db.ExecContext(ctx, query, path, language)
	if err != nil {
		return 0, fmt.Errorf("failed to insert file: %w", err)
	}

	id, _ := res.LastInsertId()
	return id, nil
}