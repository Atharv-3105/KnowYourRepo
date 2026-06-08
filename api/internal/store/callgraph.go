package store 

import (
	"context"
	"fmt"
)

type CallEdge struct {
	CallerSymbol  string 
	CallerFilePath string 
	CalleeSymbol  string 
}


func (s *Store) InsertCallEdge(ctx context.Context, edge CallEdge)error{

	query := `
	INSERT INTO call_edges (
		caller_symbol,
		caller_file_path,
		callee_symbol
	)
	VALUES (?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query, edge.CallerSymbol, edge.CallerFilePath, edge.CalleeSymbol)

	if err != nil {
		return fmt.Errorf("failed to insert call edges: %w", err)
	}

	return nil

}