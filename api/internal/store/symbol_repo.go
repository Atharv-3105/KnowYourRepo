package store

import (
	"fmt"
	"context"
)

type Symbol struct {
	ID      int64
	FileID  int64
	Name    string 
	Type    string 
	StartLine  int 
	EndLine    int 
}


func(s *Store) InsertSymbol(ctx context.Context, sym Symbol) (int64, error) {
	query := `
	INSERT INTO symbols (file_id, name, type, start_line, end_line)
	VALUES  (?, ?, ?, ?, ?)
	`

	res, err := s.db.Exec(query,
			sym.FileID,
		    sym.Name,
			sym.Type,
			sym.StartLine,
			sym.EndLine,)
	
	if err != nil {
		return 0, fmt.Errorf("failed to insert symbol: %w", err)
	}

	return res.LastInsertId()
}