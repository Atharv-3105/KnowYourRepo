//This file only exposes the raw database queries required to build the Architecture Context or Metadata Context
package store 

import (
	"context"
)

type ArchitectureFile struct{
	ID		int64
	RepoID	string
	Path	string
	Langauge	string
}

type ArchitectureSymbol struct {
	ID		int64
	FileID	int64 
	Name	string 
	Type 	string 
	StartLine int 
	EndLine   int 
}

type ArchitectureCallEdge  struct{
	CallerSymbol		string 
	CallerFilePath		string 
	CalleeeSymbol		string 
}

func(s *Store) GetFiles(ctx context.Context, repoID string) ([]ArchitectureFile, error) {
	query :=  `
		SELECT id, repo_id, path, language
		FROM files 
		WHERE repo_id = $1
		ORDER BY path
	`

	rows, err := s.db.QueryContext(ctx, query, repoID)

	if err != nil {
		return nil, err 
	}

	defer rows.Close()

	var files []ArchitectureFile

	for rows.Next() {

		var file ArchitectureFile

		err := rows.Scan(&file.ID, &file.RepoID, &file.Path, &file.Langauge)

		if err != nil {
			return nil, err 
		}

		//append the queried file into our files slice
		files = append(files, file)
	}

	return files, rows.Err()
}

//Function to query Symbols from the DB
func(s *Store) GetSymbols(ctx context.Context, repoID string) ([]ArchitectureSymbol, error) {

	query := `
		SELECT s.id, s.file_id, s.name, s.type, s.start_line, s.end_line
		FROM symbols s
		INNER JOIN files f ON s.file_id = f.id
		WHERE f.repo_id = $1
		ORDER BY f.path, s.start_line
	`

	rows, err := s.db.QueryContext(ctx, query, repoID)

	if  err != nil {
		return nil, err
	}

	defer rows.Close()

	var symbols []ArchitectureSymbol

	for rows.Next() {

		var symbol ArchitectureSymbol

		err := rows.Scan(&symbol.ID, &symbol.FileID, &symbol.Name, &symbol.Type, &symbol.StartLine, &symbol.EndLine)

		if err != nil {
			return nil, err
		}

		symbols = append(symbols, symbol)
	}

	return symbols, rows.Err()
}

//Function to query call-edges from the DB
func(s *Store) GetCallEdges(ctx context.Context, repoID string) ([]ArchitectureCallEdge, error) {

	query := `
		SELECT caller_symbol, caller_file_path, callee_symbol
		FROM call_edges
		WHERE repo_id = $1
		ORDER BY caller_symbol
	`

	rows, err := s.db.QueryContext(ctx, query, repoID)

	if err != nil{
		return nil, err
	}

	defer rows.Close()

	var edges []ArchitectureCallEdge

	for rows.Next() {

		var edge ArchitectureCallEdge

		err := rows.Scan(&edge.CallerSymbol, &edge.CallerFilePath, &edge.CalleeeSymbol)

		if err != nil {
			return nil, err
		}

		edges = append(edges, edge)
	}

	return edges, rows.Err()
}

//Function to get the Language from files table
func(s *Store) GetLanguages(ctx context.Context, repoID string) ([]string, error) {

	query := `
		SELECT DISTINCT language
		FROM files
		WHERE repo_id = $1
		ORDER BY language
	`

	rows, err := s.db.QueryContext(ctx, query, repoID)

	if err != nil{
		return nil, err 
	}

	defer rows.Close()

	var languages []string 
	
	for rows.Next() {

		var lang string 

		if err := rows.Scan(&lang); err != nil {
			return nil, err 
		}

		languages = append(languages, lang)
	}

	return languages, rows.Err()
}