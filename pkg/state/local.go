package state

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type LocalBackend struct{}

func (b *LocalBackend) Load(projectDir string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, ".vela", "state.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &s, nil
}

func (b *LocalBackend) Save(projectDir string, s *State) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	stateDir := filepath.Join(projectDir, ".vela")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("create .vela directory: %w", err)
	}
	return os.WriteFile(filepath.Join(stateDir, "state.yaml"), data, 0644)
}
