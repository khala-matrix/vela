package inspect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan_NextjsFastapiPg(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.WriteFile(filepath.Join(dir, "frontend", "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644)
	os.WriteFile(filepath.Join(dir, "backend", "requirements.txt"), []byte("fastapi\nuvicorn\nsqlalchemy\nasyncpg\n"), 0644)
	os.WriteFile(filepath.Join(dir, "frontend", "next.config.ts"), []byte("export default {}"), 0644)

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if result.DetectedStack != StackNextjsFastapiPg {
		t.Errorf("expected nextjs-fastapi-pg, got %s", result.DetectedStack)
	}
	if !result.HasPackageJSON {
		t.Error("expected HasPackageJSON to be true")
	}
	if !result.HasRequirements {
		t.Error("expected HasRequirements to be true")
	}
	if !result.HasNextConfig {
		t.Error("expected HasNextConfig to be true")
	}
}

func TestScan_NextjsFastapi(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.WriteFile(filepath.Join(dir, "frontend", "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644)
	os.WriteFile(filepath.Join(dir, "backend", "requirements.txt"), []byte("fastapi\nuvicorn\n"), 0644)

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if result.DetectedStack != StackNextjsFastapi {
		t.Errorf("expected nextjs-fastapi, got %s", result.DetectedStack)
	}
}

func TestScan_StaticSite(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "public"), 0755)
	os.WriteFile(filepath.Join(dir, "public", "index.html"), []byte("<html></html>"), 0644)
	os.WriteFile(filepath.Join(dir, "nginx.conf"), []byte("server {}"), 0644)

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if result.DetectedStack != StackStaticSite {
		t.Errorf("expected static-site, got %s", result.DetectedStack)
	}
}

func TestScan_Unsupported(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if result.DetectedStack != StackUnsupported {
		t.Errorf("expected unsupported, got %s", result.DetectedStack)
	}
}

func TestScan_NonexistentDir(t *testing.T) {
	_, err := Scan("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
