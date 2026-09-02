package ingestion 

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

//HashFile computes the SHA-256 hex digest of a file's contents, used for incremental re-indexing to detect
//which files actually changed
func HashFile(path string) (string, error){
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file for hashing: %w",err)
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:]), nil 
}