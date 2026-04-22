package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mars/vela/pkg/state"
)

func Find(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(filepath.Join(dir, ".vela"))
	if err == nil && info.IsDir() {
		return dir, nil
	}
	return "", fmt.Errorf("not a vela project — run 'vela create' first or cd into a project directory")
}

func Init(projectDir, name, namespace string) error {
	s := &state.State{
		Name:      name,
		Namespace: namespace,
		Status:    state.StatusCreated,
	}
	b := &state.LocalBackend{}
	return b.Save(projectDir, s)
}

func ChartDir(projectDir string) string {
	return filepath.Join(projectDir, ".vela", "chart")
}
