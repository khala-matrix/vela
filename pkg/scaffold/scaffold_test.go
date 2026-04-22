package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderSkeleton_NextjsFastapi(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "myapp")

	params := Params{
		Name:     "myapp",
		Registry: "registry.example.com/ns",
		Domain:   "example.com",
	}

	if err := RenderSkeleton("nextjs-fastapi", params, outDir); err != nil {
		t.Fatalf("RenderSkeleton failed: %v", err)
	}

	expectedFiles := []string{
		"tech-stack.yaml",
		"build.sh",
		"backend/Dockerfile",
		"backend/main.py",
		"backend/requirements.txt",
		"frontend/Dockerfile",
		"frontend/package.json",
		"frontend/tsconfig.json",
		"frontend/postcss.config.mjs",
		"frontend/next.config.ts",
		"frontend/src/app/globals.css",
		"frontend/src/app/layout.tsx",
		"frontend/src/app/page.tsx",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s not found: %v", f, err)
		}
	}

	techStack, _ := os.ReadFile(filepath.Join(outDir, "tech-stack.yaml"))
	content := string(techStack)
	if !strings.Contains(content, "name: myapp") {
		t.Error("tech-stack.yaml missing project name")
	}
	if !strings.Contains(content, "registry.example.com/ns/myapp-backend:latest") {
		t.Error("tech-stack.yaml missing backend image")
	}
	if !strings.Contains(content, "path: /myapp/api") {
		t.Error("tech-stack.yaml missing backend ingress path")
	}
	if !strings.Contains(content, "path: /myapp") {
		t.Error("tech-stack.yaml missing frontend ingress path")
	}

	nextConfig, _ := os.ReadFile(filepath.Join(outDir, "frontend", "next.config.ts"))
	ncContent := string(nextConfig)
	if !strings.Contains(ncContent, `basePath: "/myapp"`) {
		t.Error("next.config.ts missing basePath")
	}
	if !strings.Contains(ncContent, "myapp-myapp-backend:8000") {
		t.Error("next.config.ts missing backend service URL")
	}

	buildSh, _ := os.ReadFile(filepath.Join(outDir, "build.sh"))
	if !strings.Contains(string(buildSh), "registry.example.com/ns") {
		t.Error("build.sh missing registry")
	}

	info, _ := os.Stat(filepath.Join(outDir, "build.sh"))
	if info.Mode().Perm()&0100 == 0 {
		t.Error("build.sh is not executable")
	}
}

func TestRenderSkeleton_StaticSite(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "mysite")

	params := Params{
		Name:     "mysite",
		Registry: "registry.example.com/ns",
		Domain:   "example.com",
	}

	if err := RenderSkeleton("static-site", params, outDir); err != nil {
		t.Fatalf("RenderSkeleton failed: %v", err)
	}

	expectedFiles := []string{
		"tech-stack.yaml",
		"build.sh",
		"Dockerfile",
		"nginx.conf",
		"public/index.html",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s not found: %v", f, err)
		}
	}

	html, _ := os.ReadFile(filepath.Join(outDir, "public", "index.html"))
	if !strings.Contains(string(html), "<title>mysite</title>") {
		t.Error("index.html missing project name in title")
	}
}

func TestRenderSkeleton_InvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	err := RenderSkeleton("nonexistent-template", Params{Name: "x"}, dir)
	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
}
