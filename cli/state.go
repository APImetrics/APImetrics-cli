package cli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// State holds user-mutable CLI state persisted between runs.
type State struct {
	ProjectID string `json:"project_id,omitempty"`
}

func loadState() State {
	filename := filepath.Join(viper.GetString("config-directory"), "state.json")
	b, err := os.ReadFile(filename)
	if err != nil {
		return State{}
	}
	var s State
	json.Unmarshal(b, &s)
	return s
}

// SaveState writes the user state to disk.
func SaveState(s State) error {
	filename := filepath.Join(viper.GetString("config-directory"), "state.json")
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, b, 0600)
}
