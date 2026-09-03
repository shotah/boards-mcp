package main

import (
	"encoding/json"
	"io"
)

func writeHostManifest(w io.Writer) error {
	return json.NewEncoder(w).Encode(map[string]any{
		"name":              "boards",
		"command":           "boards-mcp",
		"env_keys":          []string{"BOARDS_AUTHOR"},
		"optional_env_keys": []string{"BOARDS_ROLE", "BOARDS_PATH", "BOARDS_WRITES_PER_DAY"},
		"blurb":             "Shared corkboard. Set BOARDS_AUTHOR to this crane slug; bind BOARDS_PATH.",
	})
}
