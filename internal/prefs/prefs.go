package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// File is the persisted UI choices. It cannot change cleanup rules.
type File struct {
	Choices   map[string]bool `json:"choices"`
	Theme     string          `json:"theme,omitempty"`
	LogKeepMB int             `json:"logKeepMB,omitempty"`
}

func ClampLogKeepMB(n int) int {
	switch n {
	case 1, 2, 5:
		return n
	default:
		return 2
	}
}

// Path is %LOCALAPPDATA%\CClear\settings.json, or dir/settings.json in tests.
func Path(base string) string {
	return filepath.Join(base, "CClear", "settings.json")
}

func Load(base string) File {
	data, err := os.ReadFile(Path(base))
	if err != nil {
		return File{Choices: map[string]bool{}}
	}
	var file File
	if json.Unmarshal(data, &file) != nil || file.Choices == nil {
		return File{Choices: map[string]bool{}}
	}
	file.LogKeepMB = ClampLogKeepMB(file.LogKeepMB)
	if file.Theme != "dark" {
		file.Theme = "light"
	}
	return file
}

func Save(base string, file File) error {
	dir := filepath.Dir(Path(base))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if file.Choices == nil {
		file.Choices = map[string]bool{}
	}
	file.LogKeepMB = ClampLogKeepMB(file.LogKeepMB)
	if file.Theme != "dark" {
		file.Theme = "light"
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(base), data, 0o600)
}
