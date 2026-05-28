package store 

import "context"

func(s *Store) GetOutgoingCalls(ctx context.Context, caller string) ([]CallEdge, error){

	query := `
	SELECT 
		caller_symbol,callee_symbol
	FROM call_edges
	WHERE caller_symbol = ?
	`

	rows, err := s.db.QueryContext(ctx, query, caller)

	if err != nil {
		return nil, err 
	}

	defer rows.Close()

	var edges []CallEdge

	for rows.Next() {

		var edge CallEdge

		err := rows.Scan(&edge.CallerSymbol, &edge.CalleeSymbol)

		if err != nil {
			continue
		}

		edges = append(edges, edge)
	}

	return edges, nil
}

func(s *Store) GetIncomingCalls(ctx context.Context, callee string) ([]CallEdge, error){

	query := `
	SELECT
		caller_symbol,
		callee_symbol
	FROM call_edges
	WHERE callee_symbol = ?
	`

	rows, err := s.db.QueryContext(ctx, query, callee)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var edges []CallEdge

	for rows.Next() {

		var edge CallEdge

		err := rows.Scan(&edge.CallerSymbol, &edge.CalleeSymbol)

		if err != nil {
			continue	
		}

		edges = append(edges, edge)
	}

	return edges, nil
}