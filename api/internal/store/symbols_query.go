package store

import (
	"context"
	"fmt"
)

// SymbolEntry is a single, fully-resolved symbol row for a repo's real
// symbol index - unlike architecture.ArchitectureSymbol (used internally
// to build the heuristic Architecture components/entrypoints subset),
// this carries everything a symbol-browsing UI needs in one row: no
// second lookup against files required.
type SymbolEntry struct {
	Name      string
	Type      string
	Language  string
	FilePath  string
	StartLine int
	EndLine   int
}

// ListSymbols returns every symbol in a repo, optionally filtered to names
// containing search (case-insensitive substring). This is the repo's real,
// complete symbol index - not the name-suffix-heuristic subset that
// architecture.DetectComponents produces for the Architecture view.
func (s *Store) ListSymbols(ctx context.Context, repoID, search string) ([]SymbolEntry, error) {

	query := `
		SELECT sym.name, sym.type, f.language, f.path, sym.start_line, sym.end_line
		FROM symbols sym
		JOIN files f ON sym.file_id = f.id
		WHERE f.repo_id = $1
		  AND ($2 = '' OR sym.name ILIKE '%' || $2 || '%')
		ORDER BY f.path, sym.start_line
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, search)
	if err != nil {
		return nil, fmt.Errorf("failed to list symbols: %w", err)
	}
	defer rows.Close()

	// Non-nil so a repo/search combination with zero matches marshals as
	// `[]`, not JSON `null` - same reasoning as architecture.go's fix.
	symbols := []SymbolEntry{}

	for rows.Next() {

		var sym SymbolEntry

		if err := rows.Scan(&sym.Name, &sym.Type, &sym.Language, &sym.FilePath, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, fmt.Errorf("failed to scan symbol entry: %w", err)
		}

		symbols = append(symbols, sym)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate symbol entries: %w", err)
	}

	return symbols, nil
}
