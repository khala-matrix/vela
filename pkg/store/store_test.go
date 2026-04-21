package store

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadApp(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	meta := AppMeta{
		Name:          "myapp",
		Namespace:     "default",
		TechStackPath: "/path/to/tech-stack.yaml",
	}

	if err := s.Save(meta); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := s.Load("myapp")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Name != "myapp" {
		t.Errorf("expected name myapp, got %s", loaded.Name)
	}
	if loaded.TechStackPath != "/path/to/tech-stack.yaml" {
		t.Errorf("expected techstack path, got %s", loaded.TechStackPath)
	}
}

func TestLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	_, err := s.Load("noapp")
	if err == nil {
		t.Fatal("expected error loading nonexistent app")
	}
}

func TestListApps(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	s.Save(AppMeta{Name: "app1", Namespace: "default"})
	s.Save(AppMeta{Name: "app2", Namespace: "default"})

	apps, err := s.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestDeleteApp(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	s.Save(AppMeta{Name: "myapp", Namespace: "default"})
	if err := s.Delete("myapp"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := s.Load("myapp")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	if s.Exists("myapp") {
		t.Fatal("should not exist yet")
	}

	s.Save(AppMeta{Name: "myapp", Namespace: "default"})

	if !s.Exists("myapp") {
		t.Fatal("should exist after save")
	}
}

func TestChartDir(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	expected := filepath.Join(dir, "myapp", "chart")
	if got := s.ChartDir("myapp"); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
