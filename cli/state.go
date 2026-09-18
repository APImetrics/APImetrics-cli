package cli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// State holds user-mutable CLI state persisted between runs.
type State struct {
	ProjectID   string `json:"project_id,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	OrgName     string `json:"org_name,omitempty"`
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

// projectIDOverride is the project selected for this run with the
// `--project-id` flag or the `<APP_NAME>_PROJECT_ID` environment variable. It
// takes precedence over the saved state and is not persisted.
var projectIDOverride string

// initProjectID resolves the project override. Called from Run once the global
// flags are parsed, before applyCredentialOverrides sets the project header.
func initProjectID(appName string) {
	projectIDOverride, _ = GlobalFlags.GetString("project-id")
	if projectIDOverride == "" {
		projectIDOverride = os.Getenv(envVarName(appName, "_PROJECT_ID"))
	}
}

// activeProjectID returns the project to send with requests: the override for
// this run if there is one, otherwise the project saved by `project select`.
func activeProjectID() string {
	if projectIDOverride != "" {
		return projectIDOverride
	}
	return loadState().ProjectID
}
