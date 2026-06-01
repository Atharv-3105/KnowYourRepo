package representation

import (
	"encoding/json"
	"os"
)

func SaveRepository(repo Repository, path string) error {

	data, err := json.MarshalIndent(repo, "", " ")

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

