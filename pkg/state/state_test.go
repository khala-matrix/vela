package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalBackend_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	velaDir := filepath.Join(dir, ".vela")
	os.MkdirAll(velaDir, 0755)

	b := &LocalBackend{}
	s := &State{
		Name:      "myapp",
		Namespace: "default",
		Status:    StatusCreated,
	}

	if err := b.Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := b.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Name != "myapp" {
		t.Errorf("expected name myapp, got %s", loaded.Name)
	}
	if loaded.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", loaded.Namespace)
	}
	if loaded.Status != StatusCreated {
		t.Errorf("expected status created, got %s", loaded.Status)
	}
}

func TestLocalBackend_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	b := &LocalBackend{}

	_, err := b.Load(dir)
	if err == nil {
		t.Fatal("expected error loading from directory without .vela/state.yaml")
	}
}

func TestLocalBackend_SaveWithServices(t *testing.T) {
	dir := t.TempDir()
	velaDir := filepath.Join(dir, ".vela")
	os.MkdirAll(velaDir, 0755)

	b := &LocalBackend{}
	s := &State{
		Name:      "myapp",
		Namespace: "prod",
		Cluster:   "https://10.0.0.1:6443",
		Revision:  3,
		Status:    StatusDeployed,
		Services: map[string]ServiceState{
			"api": {Image: "reg/api:v1", IngressPath: "/myapp/api"},
			"web": {Image: "reg/web:v1", IngressPath: "/myapp"},
		},
	}

	if err := b.Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := b.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Revision != 3 {
		t.Errorf("expected revision 3, got %d", loaded.Revision)
	}
	if loaded.Cluster != "https://10.0.0.1:6443" {
		t.Errorf("expected cluster, got %s", loaded.Cluster)
	}
	if len(loaded.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(loaded.Services))
	}
	if loaded.Services["api"].IngressPath != "/myapp/api" {
		t.Errorf("expected api ingress path /myapp/api, got %s", loaded.Services["api"].IngressPath)
	}
}
