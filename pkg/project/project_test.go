package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFind_InProjectRoot(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".vela"), 0755)

	got, err := Find(dir)
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
}

func TestFind_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := Find(dir)
	if err == nil {
		t.Fatal("expected error when no .vela/ exists")
	}
}

func TestInit_CreatesVelaDir(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "myapp")
	os.MkdirAll(projectDir, 0755)

	if err := Init(projectDir, "myapp", "default"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(projectDir, ".vela"))
	if err != nil {
		t.Fatalf(".vela dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".vela is not a directory")
	}

	stateData, err := os.ReadFile(filepath.Join(projectDir, ".vela", "state.yaml"))
	if err != nil {
		t.Fatalf("state.yaml not created: %v", err)
	}
	if !strings.Contains(string(stateData), "name: myapp") {
		t.Error("state.yaml missing project name")
	}
	if !strings.Contains(string(stateData), "status: created") {
		t.Error("state.yaml missing status")
	}
}

func TestChartDir(t *testing.T) {
	dir := t.TempDir()
	expected := filepath.Join(dir, ".vela", "chart")
	if got := ChartDir(dir); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
