package store 

import (
	"context"
)

type Repository struct {
	ID    	string 
	RepoURL	string 
}

func(s *Store) InsertRepository(ctx context.Context, repo Repository) error{
	query := `
	INSERT INTO repositories(id, repo_url)
	VALUES(?,?)`

	_, err := s.db.ExecContext(ctx, query, repo.ID, repo.RepoURL)

	return err 
}