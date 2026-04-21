package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type AppMeta struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	TechStackPath string `json:"techStackPath,omitempty"`
}

type Store struct {
	baseDir string
}

func New(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func DefaultBaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vela")
}

func (s *Store) Save(meta AppMeta) error {
	appDir := filepath.Join(s.baseDir, meta.Name)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("create app directory: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	return os.WriteFile(filepath.Join(appDir, "meta.json"), data, 0644)
}

func (s *Store) Load(name string) (AppMeta, error) {
	data, err := os.ReadFile(filepath.Join(s.baseDir, name, "meta.json"))
	if err != nil {
		return AppMeta{}, fmt.Errorf("app %q not found: %w", name, err)
	}
	var meta AppMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return AppMeta{}, fmt.Errorf("parse metadata: %w", err)
	}
	return meta, nil
}

func (s *Store) List() ([]AppMeta, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var apps []AppMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, err := s.Load(entry.Name())
		if err != nil {
			continue
		}
		apps = append(apps, meta)
	}
	return apps, nil
}

func (s *Store) Delete(name string) error {
	appDir := filepath.Join(s.baseDir, name)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		return fmt.Errorf("app %q not found", name)
	}
	return os.RemoveAll(appDir)
}

func (s *Store) Exists(name string) bool {
	_, err := os.Stat(filepath.Join(s.baseDir, name, "meta.json"))
	return err == nil
}

func (s *Store) ChartDir(name string) string {
	return filepath.Join(s.baseDir, name, "chart")
}

func (s *Store) AppDir(name string) string {
	return filepath.Join(s.baseDir, name)
}
