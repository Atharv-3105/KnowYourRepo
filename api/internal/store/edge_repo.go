package store   

import "fmt"

type Edge struct {
	ID      int64
	FromSymbolID   int64
	ToSymbolID     int64
	Type          string 
}


func(s *Store) InsertEdge(e Edge) error {
	query := `
	INSERT INTO edges (from_symbol_id, to_symbol_id, type)
	VALUES  (?, ?, ?)`

	_, err := s.db.Exec(query, e.FromSymbolID, e.ToSymbolID, e.Type)
	if err != nil {
		return fmt.Errorf("failed to insert edge: %w",err)
	}

	return nil
}

