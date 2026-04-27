package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const updateStateFileName = "update_state.json"

type UpdateState struct {
	LastCheckAt *time.Time `json:"last_check_at,omitempty"`
}

func UpdateStatePath(dir string) string {
	return filepath.Join(dir, updateStateFileName)
}

func ReadUpdateState(dir string) (UpdateState, error) {
	path := UpdateStatePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return UpdateState{}, nil
		}
		return UpdateState{}, err
	}

	var s UpdateState
	if err := json.Unmarshal(data, &s); err != nil {
		// If the file is corrupted, treat as empty state rather than breaking the CLI.
		return UpdateState{}, nil
	}
	return s, nil
}

func WriteUpdateState(dir string, s UpdateState) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(UpdateStatePath(dir), data, 0o600)
}

