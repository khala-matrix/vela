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
		Name:         "myapp",
		Namespace:    "sandbox",
		Registry:     "registry.example.com/ns",
		Domain:       "example.com",
		BaseRegistry: "harbor.example.com/library",
		BuildTool:    "docker",
		BuildCmd:     "build",
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
	if !strings.Contains(content, "path: /sandbox/myapp/api") {
		t.Error("tech-stack.yaml missing backend ingress path")
	}
	if !strings.Contains(content, "path: /sandbox/myapp") {
		t.Error("tech-stack.yaml missing frontend ingress path")
	}

	nextConfig, _ := os.ReadFile(filepath.Join(outDir, "frontend", "next.config.ts"))
	ncContent := string(nextConfig)
	if !strings.Contains(ncContent, `basePath: "/sandbox/myapp"`) {
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

	backendDockerfile, _ := os.ReadFile(filepath.Join(outDir, "backend", "Dockerfile"))
	if !strings.Contains(string(backendDockerfile), "harbor.example.com/library/python:3.12-slim-bookworm") {
		t.Error("backend Dockerfile missing base registry prefix")
	}

	frontendDockerfile, _ := os.ReadFile(filepath.Join(outDir, "frontend", "Dockerfile"))
	if !strings.Contains(string(frontendDockerfile), "harbor.example.com/library/node:22-alpine") {
		t.Error("frontend Dockerfile missing base registry prefix")
	}
}

func TestRenderSkeleton_NextjsFastapi_NoBaseRegistry(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "myapp")

	params := Params{
		Name:      "myapp",
		Registry:  "registry.example.com/ns",
		Domain:    "example.com",
		BuildTool: "docker",
		BuildCmd:  "build",
	}

	if err := RenderSkeleton("nextjs-fastapi", params, outDir); err != nil {
		t.Fatalf("RenderSkeleton failed: %v", err)
	}

	backendDockerfile, _ := os.ReadFile(filepath.Join(outDir, "backend", "Dockerfile"))
	if !strings.Contains(string(backendDockerfile), "FROM python:3.12-slim-bookworm") {
		t.Error("backend Dockerfile should use plain python image without base registry")
	}

	frontendDockerfile, _ := os.ReadFile(filepath.Join(outDir, "frontend", "Dockerfile"))
	if !strings.Contains(string(frontendDockerfile), "FROM node:22-alpine") {
		t.Error("frontend Dockerfile should use plain node image without base registry")
	}
}

func TestRenderSkeleton_StaticSite(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "mysite")

	params := Params{
		Name:      "mysite",
		Registry:  "registry.example.com/ns",
		Domain:    "example.com",
		BuildTool: "docker",
		BuildCmd:  "build",
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

func TestRenderSkeleton_NextjsFastapiPg(t *testing.T) {
	outDir := t.TempDir()
	params := Params{
		Name:            "testapp",
		Namespace:       "sandbox",
		Registry:        "harbor.example.com/ns",
		Domain:          "example.com",
		BaseRegistry:    "harbor.example.com/baselibrary",
		DBPassword:      "testpass123",
		DBImageRegistry: "harbor.example.com/tools",
		BuildTool:       "docker",
		BuildCmd:        "build",
	}

	if err := RenderSkeleton("nextjs-fastapi-pg", params, outDir); err != nil {
		t.Fatalf("RenderSkeleton failed: %v", err)
	}

	mainPy, err := os.ReadFile(filepath.Join(outDir, "backend", "main.py"))
	if err != nil {
		t.Fatalf("backend/main.py missing: %v", err)
	}
	mainContent := string(mainPy)
	if !strings.Contains(mainContent, "testapp-postgresql:5432/testapp") {
		t.Error("main.py missing correct DATABASE_URL")
	}
	if !strings.Contains(mainContent, "testpass123") {
		t.Error("main.py missing DB password")
	}
	if !strings.Contains(mainContent, "/sandbox/testapp/api") {
		t.Error("main.py missing correct API prefix")
	}

	pageTsx, err := os.ReadFile(filepath.Join(outDir, "frontend", "src", "app", "page.tsx"))
	if err != nil {
		t.Fatalf("page.tsx missing: %v", err)
	}
	if !strings.Contains(string(pageTsx), "/api/db-health") {
		t.Error("page.tsx missing db-health check")
	}
	if !strings.Contains(string(pageTsx), "/api/todos") {
		t.Error("page.tsx missing todos API calls")
	}

	techStack, err := os.ReadFile(filepath.Join(outDir, "tech-stack.yaml"))
	if err != nil {
		t.Fatalf("tech-stack.yaml missing: %v", err)
	}
	tsContent := string(techStack)
	if !strings.Contains(tsContent, "postgresql:") {
		t.Error("tech-stack.yaml missing postgresql dependency")
	}
	if !strings.Contains(tsContent, "testpass123") {
		t.Error("tech-stack.yaml missing DB password")
	}
	if !strings.Contains(tsContent, "imageRegistry:") {
		t.Error("tech-stack.yaml missing imageRegistry")
	}

	buildSh := filepath.Join(outDir, "build.sh")
	info, err := os.Stat(buildSh)
	if err != nil {
		t.Fatalf("build.sh missing: %v", err)
	}
	if info.Mode()&0100 == 0 {
		t.Error("build.sh is not executable")
	}
}

func TestRenderSkeleton_BuildahBuilder(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "myapp")

	params := Params{
		Name:      "myapp",
		Namespace: "sandbox",
		Registry:  "registry.example.com/ns",
		Domain:    "example.com",
		BuildTool: "buildah",
		BuildCmd:  "bud",
	}

	if err := RenderSkeleton("nextjs-fastapi", params, outDir); err != nil {
		t.Fatalf("RenderSkeleton failed: %v", err)
	}

	buildSh, _ := os.ReadFile(filepath.Join(outDir, "build.sh"))
	content := string(buildSh)
	if !strings.Contains(content, "buildah bud") {
		t.Error("build.sh should use 'buildah bud'")
	}
	if !strings.Contains(content, "buildah push") {
		t.Error("build.sh should use 'buildah push'")
	}
	if !strings.Contains(content, "buildah login") {
		t.Error("build.sh should use 'buildah login'")
	}
	if strings.Contains(content, "docker") {
		t.Error("build.sh should not contain 'docker' when using buildah")
	}
}

func TestRenderSkeleton_InvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	err := RenderSkeleton("nonexistent-template", Params{Name: "x"}, dir)
	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
}
