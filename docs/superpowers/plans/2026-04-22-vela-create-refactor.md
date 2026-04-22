# Vela CLI Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor Vela CLI to flatten subcommands, replace `~/.vela/` global store with project-local `.vela/` directory, and make `vela create` generate complete project skeletons from embedded templates.

**Architecture:** New `pkg/state` handles `.vela/state.yaml` read/write via a `Backend` interface (only `LocalBackend` for now). New `pkg/project` detects `.vela/` directories. `pkg/scaffold` is rewritten to walk embedded skeleton directory trees instead of single YAML templates. All `cmd/app/*` subcommands are moved to flat `cmd/*.go` files. Old `cmd/configure/`, `cmd/update/`, `cmd/app/`, and `pkg/store` are deleted.

**Tech Stack:** Go 1.25, Cobra, Bubbletea, Helm Go SDK, `//go:embed`, `gopkg.in/yaml.v3`

---

## File Structure

### New files

| File | Responsibility |
|------|---------------|
| `pkg/state/state.go` | `State` struct, `Backend` interface, status constants |
| `pkg/state/local.go` | `LocalBackend` — reads/writes `.vela/state.yaml` |
| `pkg/state/state_test.go` | Tests for State + LocalBackend |
| `pkg/project/project.go` | `Find()` — detect `.vela/` in cwd; `Init()` — create `.vela/` dir + initial state |
| `pkg/project/project_test.go` | Tests for Find and Init |
| `pkg/scaffold/scaffold.go` | Rewritten: `RenderSkeleton()` walks embedded dir tree, `Templates` registry |
| `pkg/scaffold/scaffold_test.go` | Rewritten: tests skeleton generation for all templates |
| `pkg/scaffold/skeletons/nextjs-fastapi/` | 12 `.tmpl` files for Next.js + FastAPI skeleton |
| `pkg/scaffold/skeletons/static-site/` | 5 `.tmpl` files for static site skeleton |
| `cmd/create.go` | `vela create` — TUI + skeleton generation |
| `cmd/deploy.go` | `vela deploy` — chart gen + helm install/upgrade + state sync |
| `cmd/status.go` | `vela status` — pod status + state sync |
| `cmd/logs.go` | `vela logs` — log streaming |
| `cmd/delete.go` | `vela delete` — helm uninstall + state update |
| `cmd/list.go` | `vela list` — cluster-wide query |
| `cmd/configure.go` | `vela configure` — self-update (moved from `cmd/update/`) |

### Modified files

| File | Change |
|------|--------|
| `cmd/root.go` | Remove `cmd/app`, `cmd/configure`, `cmd/update` imports; register new flat commands |
| `cmd/version.go` | No change (stays as-is) |

### Deleted files/directories

| Path | Reason |
|------|--------|
| `cmd/app/` (entire directory) | Replaced by flat `cmd/*.go` |
| `cmd/configure/` (entire directory) | Merged into `cmd/create.go` (TUI) and `cmd/configure.go` (self-update) |
| `cmd/update/` (entire directory) | Merged into `cmd/configure.go` |
| `pkg/store/` (entire directory) | Replaced by `pkg/state/` + `pkg/project/` |
| `pkg/scaffold/templates/` (entire directory) | Replaced by `pkg/scaffold/skeletons/` |

### Unchanged packages

| Package | Notes |
|---------|-------|
| `pkg/config/` | No changes — tech-stack.yaml parsing stays the same |
| `pkg/chart/` | No changes — chart generation stays the same |
| `pkg/helm/` | No changes — helm SDK wrapper stays the same |
| `pkg/kube/` | No changes — k8s client stays the same |

---

## Task Dependency Order

```
Task 1 (state pkg) → Task 4 (deploy), Task 5 (status), Task 7 (delete)
Task 2 (project pkg) → Task 3 (create), Task 4 (deploy), Task 5 (status)
Task 3 (scaffold + create) → can start skeleton files (Task 3a) in parallel with Tasks 1-2
Task 8 (root.go rewire) → must be last, after all commands exist
Task 9 (delete old code) → must be after Task 8
```

---

### Task 1: State Package

**Files:**
- Create: `pkg/state/state.go`
- Create: `pkg/state/local.go`
- Create: `pkg/state/state_test.go`

- [ ] **Step 1: Write the failing test for State struct and LocalBackend**

```go
// pkg/state/state_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/state/ -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement State struct and Backend interface**

```go
// pkg/state/state.go
package state

const (
	StatusCreated  = "created"
	StatusDeployed = "deployed"
	StatusFailed   = "failed"
	StatusDeleted  = "deleted"
)

type State struct {
	Name         string                  `yaml:"name"`
	Namespace    string                  `yaml:"namespace"`
	Cluster      string                  `yaml:"cluster,omitempty"`
	LastDeployed string                  `yaml:"last_deployed,omitempty"`
	Revision     int                     `yaml:"revision,omitempty"`
	Status       string                  `yaml:"status"`
	Services     map[string]ServiceState `yaml:"services,omitempty"`
}

type ServiceState struct {
	Image       string `yaml:"image"`
	IngressPath string `yaml:"ingress_path,omitempty"`
}

type Backend interface {
	Load(projectDir string) (*State, error)
	Save(projectDir string, state *State) error
}
```

- [ ] **Step 4: Implement LocalBackend**

```go
// pkg/state/local.go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./pkg/state/ -v`
Expected: PASS — all 3 tests

- [ ] **Step 6: Commit**

```bash
git add pkg/state/
git commit -m "feat: add state package with Backend interface and LocalBackend"
```

---

### Task 2: Project Package

**Files:**
- Create: `pkg/project/project.go`
- Create: `pkg/project/project_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// pkg/project/project_test.go
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
	if !contains(string(stateData), "name: myapp") {
		t.Error("state.yaml missing project name")
	}
	if !contains(string(stateData), "status: created") {
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

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/project/ -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement project package**

```go
// pkg/project/project.go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/project/ -v`
Expected: PASS — all 4 tests

- [ ] **Step 5: Commit**

```bash
git add pkg/project/
git commit -m "feat: add project package for .vela/ directory detection and init"
```

---

### Task 3: Scaffold Rewrite — Skeleton Rendering

**Files:**
- Rewrite: `pkg/scaffold/scaffold.go`
- Rewrite: `pkg/scaffold/scaffold_test.go`
- Delete: `pkg/scaffold/templates/` (5 old YAML templates)
- Create: `pkg/scaffold/skeletons/nextjs-fastapi/` (12 files)
- Create: `pkg/scaffold/skeletons/static-site/` (5 files)

This is the largest task. It has sub-steps for creating skeleton template files, then the renderer, then the tests.

- [ ] **Step 1: Create nextjs-fastapi skeleton template files**

Create `pkg/scaffold/skeletons/nextjs-fastapi/tech-stack.yaml.tmpl`:

```yaml
name: {{ .Name }}
ingress:
  host: {{ .Domain }}
services:
  - name: {{ .Name }}-backend
    image: {{ .Registry }}/{{ .Name }}-backend:latest
    port: 8000
    env:
      - name: CORS_ORIGINS
        value: "*"
      - name: PYTHONUNBUFFERED
        value: "1"
    ingress:
      enabled: true
      path: /{{ .Name }}/api
  - name: {{ .Name }}-frontend
    image: {{ .Registry }}/{{ .Name }}-frontend:latest
    port: 3000
    ingress:
      enabled: true
      path: /{{ .Name }}
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/build.sh.tmpl`:

```bash
#!/usr/bin/env bash
set -euo pipefail

REGISTRY="${REGISTRY:-{{ .Registry }}}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"

if [ -n "${REGISTRY_USER:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; then
  echo "==> Logging in to ${REGISTRY%%/*}"
  echo "${REGISTRY_PASSWORD}" | docker login "${REGISTRY%%/*}" -u "${REGISTRY_USER}" --password-stdin
else
  echo "==> REGISTRY_USER / REGISTRY_PASSWORD not set, skipping login"
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> Building {{ .Name }}-backend (${PLATFORM})"
docker build --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}-backend:${TAG}" "${SCRIPT_DIR}/backend"

echo "==> Building {{ .Name }}-frontend (${PLATFORM})"
docker build --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}-frontend:${TAG}" "${SCRIPT_DIR}/frontend"

echo "==> Pushing images"
docker push "${REGISTRY}/{{ .Name }}-backend:${TAG}"
docker push "${REGISTRY}/{{ .Name }}-frontend:${TAG}"

echo "==> Done"
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/backend/Dockerfile.tmpl`:

```
FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/backend/main.py.tmpl`:

```python
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
import os

app = FastAPI(title="{{ .Name }} API")

app.add_middleware(
    CORSMiddleware,
    allow_origins=os.getenv("CORS_ORIGINS", "*").split(","),
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/api/health")
def health():
    return {"status": "ok"}


@app.get("/api/hello")
def hello():
    return {"message": "Hello from {{ .Name }}"}
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/backend/requirements.txt.tmpl`:

```
fastapi==0.115.12
uvicorn==0.34.2
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/Dockerfile.tmpl`:

```
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json ./
RUN npm install

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:22-alpine
WORKDIR /app
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

EXPOSE 3000
ENV HOSTNAME="0.0.0.0"
CMD ["node", "server.js"]
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/package.json.tmpl`:

```json
{
  "name": "{{ .Name }}-frontend",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start"
  },
  "dependencies": {
    "next": "^15.3.2",
    "react": "^19.1.0",
    "react-dom": "^19.1.0"
  },
  "devDependencies": {
    "@tailwindcss/postcss": "^4.1.4",
    "@types/node": "^22.15.3",
    "@types/react": "^19.1.2",
    "tailwindcss": "^4.1.4",
    "typescript": "^5.8.3"
  }
}
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/tsconfig.json.tmpl`:

```json
{
  "compilerOptions": {
    "target": "ES2017",
    "lib": ["dom", "dom.iterable", "esnext"],
    "allowJs": true,
    "skipLibCheck": true,
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "module": "esnext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "jsx": "react-jsx",
    "incremental": true,
    "plugins": [{"name": "next"}],
    "paths": {"@/*": ["./src/*"]}
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules"]
}
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/postcss.config.mjs.tmpl`:

```javascript
const config = {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};

export default config;
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/next.config.ts.tmpl`:

```typescript
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  basePath: "/{{ .Name }}",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.API_BACKEND_URL || "http://{{ .Name }}-{{ .Name }}-backend:8000"}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/src/app/globals.css.tmpl`:

```css
@import "tailwindcss";
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/src/app/layout.tsx.tmpl`:

```tsx
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "{{ .Name }}",
  description: "Deployed with Vela",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="bg-gray-950 text-gray-100 antialiased">{children}</body>
    </html>
  );
}
```

Create `pkg/scaffold/skeletons/nextjs-fastapi/frontend/src/app/page.tsx.tmpl`:

```tsx
export default function Home() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center">
      <h1 className="mb-4 text-4xl font-bold">{{ .Name }}</h1>
      <p className="text-gray-400">Deployed with Vela</p>
    </main>
  );
}
```

- [ ] **Step 2: Create static-site skeleton template files**

Create `pkg/scaffold/skeletons/static-site/tech-stack.yaml.tmpl`:

```yaml
name: {{ .Name }}
ingress:
  host: {{ .Domain }}
services:
  - name: {{ .Name }}
    image: {{ .Registry }}/{{ .Name }}:latest
    port: 80
    resources:
      cpu: 100m
      memory: 64Mi
    ingress:
      enabled: true
      path: /{{ .Name }}
```

Create `pkg/scaffold/skeletons/static-site/build.sh.tmpl`:

```bash
#!/usr/bin/env bash
set -euo pipefail

REGISTRY="${REGISTRY:-{{ .Registry }}}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"

if [ -n "${REGISTRY_USER:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; then
  echo "==> Logging in to ${REGISTRY%%/*}"
  echo "${REGISTRY_PASSWORD}" | docker login "${REGISTRY%%/*}" -u "${REGISTRY_USER}" --password-stdin
else
  echo "==> REGISTRY_USER / REGISTRY_PASSWORD not set, skipping login"
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> Building {{ .Name }} (${PLATFORM})"
docker build --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}:${TAG}" "${SCRIPT_DIR}"

echo "==> Pushing image"
docker push "${REGISTRY}/{{ .Name }}:${TAG}"

echo "==> Done"
```

Create `pkg/scaffold/skeletons/static-site/Dockerfile.tmpl`:

```
FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY public/ /usr/share/nginx/html/
EXPOSE 80
```

Create `pkg/scaffold/skeletons/static-site/nginx.conf.tmpl`:

```nginx
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

Create `pkg/scaffold/skeletons/static-site/public/index.html.tmpl`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{ .Name }}</title>
  <style>
    body { font-family: system-ui, sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #0a0a0a; color: #e5e5e5; }
    h1 { font-size: 2.5rem; font-weight: bold; }
    p { color: #888; }
  </style>
</head>
<body>
  <div style="text-align:center">
    <h1>{{ .Name }}</h1>
    <p>Deployed with Vela</p>
  </div>
</body>
</html>
```

- [ ] **Step 3: Rewrite scaffold.go — RenderSkeleton + Templates registry**

Delete old template files: `pkg/scaffold/templates/*.tmpl`

Rewrite `pkg/scaffold/scaffold.go`:

```go
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed all:skeletons
var skeletonFS embed.FS

type Template struct {
	ID          string
	Name        string
	Description string
}

type Params struct {
	Name     string
	Registry string
	Domain   string
}

var Templates = []Template{
	{ID: "nextjs-fastapi", Name: "Next.js + FastAPI", Description: "Full-stack web app — Python backend, React frontend"},
	{ID: "static-site", Name: "Static Site", Description: "Single container — nginx static files"},
}

func RenderSkeleton(templateID string, params Params, outDir string) error {
	root := filepath.Join("skeletons", templateID)

	return fs.WalkDir(skeletonFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(outDir, strings.TrimSuffix(relPath, ".tmpl"))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		content, err := skeletonFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}

		tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", path, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, params); err != nil {
			return fmt.Errorf("render template %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("create directory for %s: %w", destPath, err)
		}

		perm := os.FileMode(0644)
		if strings.HasSuffix(destPath, ".sh") {
			perm = 0755
		}

		return os.WriteFile(destPath, buf.Bytes(), perm)
	})
}
```

- [ ] **Step 4: Rewrite scaffold_test.go**

```go
// pkg/scaffold/scaffold_test.go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./pkg/scaffold/ -v`
Expected: PASS — all 3 tests

- [ ] **Step 6: Commit**

```bash
git add pkg/scaffold/
git commit -m "feat: rewrite scaffold to generate full project skeletons from embedded directory trees"
```

---

### Task 4: `vela create` Command

**Files:**
- Create: `cmd/create.go`

- [ ] **Step 1: Implement create command**

```go
// cmd/create.go
package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/scaffold"
	"github.com/spf13/cobra"
)

var (
	createTemplate string
	createRegistry string
	createDomain   string
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new project from a template",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createTemplate, "template", "t", "", "template ID (e.g. nextjs-fastapi, static-site)")
	createCmd.Flags().StringVar(&createRegistry, "registry", "", "image registry (e.g. registry.example.com/ns)")
	createCmd.Flags().StringVar(&createDomain, "domain", "", "ingress domain (e.g. example.com)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	if name != "" && createTemplate != "" && createRegistry != "" && createDomain != "" {
		return generateProject(cmd, name, createTemplate, createRegistry, createDomain)
	}

	p := tea.NewProgram(newCreateModel(name))
	m, err := p.Run()
	if err != nil {
		return err
	}

	final := m.(createModel)
	if final.cancelled {
		fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
		return nil
	}

	tmpl := scaffold.Templates[final.templateIdx]
	return generateProject(cmd, final.inputs[0], tmpl.ID, final.inputs[1], final.inputs[2])
}

func generateProject(cmd *cobra.Command, name, templateID, registry, domain string) error {
	outDir := name

	if _, err := os.Stat(outDir); err == nil {
		return fmt.Errorf("directory %q already exists", outDir)
	}

	var tmpl *scaffold.Template
	for i := range scaffold.Templates {
		if scaffold.Templates[i].ID == templateID {
			tmpl = &scaffold.Templates[i]
			break
		}
	}
	if tmpl == nil {
		ids := make([]string, len(scaffold.Templates))
		for i, t := range scaffold.Templates {
			ids[i] = t.ID
		}
		return fmt.Errorf("unknown template %q, available: %s", templateID, strings.Join(ids, ", "))
	}

	params := scaffold.Params{
		Name:     name,
		Registry: registry,
		Domain:   domain,
	}

	if err := scaffold.RenderSkeleton(templateID, params, outDir); err != nil {
		return fmt.Errorf("generate skeleton: %w", err)
	}

	ns := cmd.Flag("namespace").Value.String()
	if err := project.Init(outDir, name, ns); err != nil {
		os.RemoveAll(outDir)
		return fmt.Errorf("init project: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created project %s/\n\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "Next steps:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  cd %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  ./build.sh        # build & push images\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  vela deploy       # deploy to cluster\n")
	return nil
}

// --- TUI Model ---

type createStep int

const (
	createStepTemplate createStep = iota
	createStepInput
	createStepConfirm
)

type createModel struct {
	step        createStep
	templateIdx int
	inputIdx    int
	inputs      [3]string
	cancelled   bool
	width       int
}

var createInputLabels = [3]string{"Project name", "Image registry", "Ingress domain"}
var createInputPlaceholders = [3]string{"my-app", "registry.example.com/namespace", "example.com"}

var (
	cTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	cSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	cDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	cInputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
)

func newCreateModel(name string) createModel {
	m := createModel{}
	if name != "" {
		m.inputs[0] = name
	}
	return m
}

func (m createModel) Init() tea.Cmd {
	return nil
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "q":
			if m.step == createStepTemplate {
				m.cancelled = true
				return m, tea.Quit
			}
		case "esc":
			if m.step == createStepInput && m.inputIdx > 0 {
				m.inputIdx--
				return m, nil
			}
			if m.step == createStepInput && m.inputIdx == 0 {
				m.step = createStepTemplate
				return m, nil
			}
			if m.step == createStepConfirm {
				m.step = createStepInput
				m.inputIdx = 2
				return m, nil
			}
		}
	}

	switch m.step {
	case createStepTemplate:
		return m.updateTemplate(msg)
	case createStepInput:
		return m.updateInput(msg)
	case createStepConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

func (m createModel) updateTemplate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.templateIdx > 0 {
				m.templateIdx--
			}
		case "down", "j":
			if m.templateIdx < len(scaffold.Templates)-1 {
				m.templateIdx++
			}
		case "enter":
			m.step = createStepInput
		}
	}
	return m, nil
}

func (m createModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter":
			if m.inputs[m.inputIdx] == "" {
				m.inputs[m.inputIdx] = createInputPlaceholders[m.inputIdx]
			}
			if m.inputIdx < 2 {
				m.inputIdx++
			} else {
				m.step = createStepConfirm
			}
		case "backspace":
			if len(m.inputs[m.inputIdx]) > 0 {
				m.inputs[m.inputIdx] = m.inputs[m.inputIdx][:len(m.inputs[m.inputIdx])-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.inputs[m.inputIdx] += msg.String()
			}
		}
	}
	return m, nil
}

func (m createModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter", "y":
			return m, tea.Quit
		case "n":
			m.step = createStepInput
			m.inputIdx = 0
			m.inputs = [3]string{}
		}
	}
	return m, nil
}

func (m createModel) View() string {
	var b strings.Builder

	b.WriteString(cTitleStyle.Render("vela create"))
	b.WriteString("\n\n")

	switch m.step {
	case createStepTemplate:
		b.WriteString("Select a tech stack template:\n\n")
		for i, t := range scaffold.Templates {
			cursor := "  "
			name := cDimStyle.Render(t.Name)
			desc := cDimStyle.Render(" — " + t.Description)
			if i == m.templateIdx {
				cursor = cSelectedStyle.Render("> ")
				name = cSelectedStyle.Render(t.Name)
			}
			b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, name, desc))
		}
		b.WriteString(cDimStyle.Render("\n↑/↓ navigate • enter select • q quit"))

	case createStepInput:
		tmpl := scaffold.Templates[m.templateIdx]
		b.WriteString(fmt.Sprintf("Template: %s\n\n", cSelectedStyle.Render(tmpl.Name)))

		for i := 0; i < 3; i++ {
			label := createInputLabels[i]
			if i < m.inputIdx {
				b.WriteString(fmt.Sprintf("  %s: %s\n", cDimStyle.Render(label), cInputStyle.Render(m.inputs[i])))
			} else if i == m.inputIdx {
				val := m.inputs[i]
				placeholder := ""
				if val == "" {
					placeholder = cDimStyle.Render(createInputPlaceholders[i])
				}
				b.WriteString(fmt.Sprintf("  %s: %s%s▏\n", cPromptStyle.Render(label), cInputStyle.Render(val), placeholder))
			} else {
				b.WriteString(fmt.Sprintf("  %s:\n", cDimStyle.Render(label)))
			}
		}
		b.WriteString(cDimStyle.Render("\nenter confirm • esc back"))

	case createStepConfirm:
		tmpl := scaffold.Templates[m.templateIdx]
		b.WriteString("Review:\n\n")
		b.WriteString(fmt.Sprintf("  Template: %s\n", cSelectedStyle.Render(tmpl.Name)))
		b.WriteString(fmt.Sprintf("  Project:  %s\n", cInputStyle.Render(m.inputs[0])))
		b.WriteString(fmt.Sprintf("  Registry: %s\n", cInputStyle.Render(m.inputs[1])))
		b.WriteString(fmt.Sprintf("  Domain:   %s\n", cInputStyle.Render(m.inputs[2])))
		b.WriteString(cDimStyle.Render("\nenter/y generate • n restart • esc back"))
	}

	b.WriteString("\n")
	return b.String()
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./cmd/...` (should not be wired up yet, just check the file compiles in isolation — skip this until Task 8 wires everything together)

- [ ] **Step 3: Commit**

```bash
git add cmd/create.go
git commit -m "feat: add vela create command with TUI and skeleton generation"
```

---

### Task 5: `vela deploy` Command

**Files:**
- Create: `cmd/deploy.go`

- [ ] **Step 1: Implement deploy command**

```go
// cmd/deploy.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/mars/vela/pkg/chart"
	"github.com/mars/vela/pkg/config"
	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the application to the cluster",
	Args:  cobra.NoArgs,
	RunE:  runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	projectDir, err := project.Find(cwd)
	if err != nil {
		return err
	}

	backend := &state.LocalBackend{}
	st, err := backend.Load(projectDir)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	ts, err := config.Parse("tech-stack.yaml")
	if err != nil {
		return fmt.Errorf("parse tech-stack: %w", err)
	}

	chartDir := project.ChartDir(projectDir)
	if err := os.RemoveAll(chartDir); err != nil {
		return fmt.Errorf("clean chart dir: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return fmt.Errorf("create chart dir: %w", err)
	}

	if err := chart.Generate(ts, chartDir); err != nil {
		return fmt.Errorf("generate chart: %w", err)
	}

	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfigVal, ns)
	name := ts.ProjectName()

	if hc.ReleaseExists(name) {
		fmt.Fprintf(cmd.OutOrStdout(), "Upgrading %q in namespace %q...\n", name, ns)
		if err := hc.Upgrade(name, chartDir); err != nil {
			st.Status = state.StatusFailed
			backend.Save(projectDir, st)
			return fmt.Errorf("upgrade failed: %w", err)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Deploying %q to namespace %q...\n", name, ns)
		if err := hc.Install(name, chartDir); err != nil {
			st.Status = state.StatusFailed
			backend.Save(projectDir, st)
			return fmt.Errorf("deploy failed: %w", err)
		}
	}

	rel, _ := hc.Status(name)
	st.Status = state.StatusDeployed
	st.Namespace = ns
	st.Cluster = kubeconfigVal
	st.LastDeployed = time.Now().UTC().Format(time.RFC3339)
	if rel != nil {
		st.Revision = rel.Revision
	}

	st.Services = make(map[string]state.ServiceState)
	for _, svc := range ts.Services {
		ss := state.ServiceState{Image: svc.Image}
		if svc.Ingress != nil && svc.Ingress.Enabled {
			ss.IngressPath = svc.Ingress.Path
		}
		st.Services[svc.Name] = ss
	}
	backend.Save(projectDir, st)

	fmt.Fprintf(cmd.OutOrStdout(), "App %q deployed successfully.\n", name)
	fmt.Fprintln(cmd.OutOrStdout(), "Run 'vela status' to check deployment status.")
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/deploy.go
git commit -m "feat: add vela deploy command with state sync"
```

---

### Task 6: `vela status` Command

**Files:**
- Create: `cmd/status.go`

- [ ] **Step 1: Implement status command**

```go
// cmd/status.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/kube"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show application deployment status",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	projectDir, err := project.Find(cwd)
	if err != nil {
		return err
	}

	backend := &state.LocalBackend{}
	st, err := backend.Load(projectDir)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()

	hc := helm.New(kubeconfigVal, ns)
	rel, err := hc.Status(st.Name)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "App %q is not deployed to the cluster.\n", st.Name)
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'vela deploy' to deploy.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Release:   %s\n", rel.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Namespace: %s\n", rel.Namespace)
	fmt.Fprintf(cmd.OutOrStdout(), "Status:    %s\n", rel.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "Revision:  %d\n", rel.Revision)
	if !rel.Updated.IsZero() {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated:   %s\n", rel.Updated.Format("2006-01-02 15:04:05"))
	}

	st.Status = state.StatusDeployed
	st.Namespace = rel.Namespace
	st.Revision = rel.Revision
	backend.Save(projectDir, st)

	kc, err := kube.New(kubeconfigVal, ns)
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	pods, err := kc.GetPods(context.Background(), st.Name)
	if err != nil {
		return fmt.Errorf("get pods: %w", err)
	}

	if len(pods) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNo pods found.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nPods:")
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tREADY")
	for _, pod := range pods {
		ready := "No"
		if pod.Ready {
			ready = "Yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", pod.Name, pod.Status, ready)
	}
	w.Flush()
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/status.go
git commit -m "feat: add vela status command with state sync"
```

---

### Task 7: `vela logs` Command

**Files:**
- Create: `cmd/logs.go`

- [ ] **Step 1: Implement logs command**

```go
// cmd/logs.go
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/mars/vela/pkg/kube"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsAll    bool
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View application pod logs",
	Args:  cobra.NoArgs,
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "F", false, "follow log output")
	logsCmd.Flags().BoolVar(&logsAll, "all", false, "show logs from all pods")
}

func runLogs(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	projectDir, err := project.Find(cwd)
	if err != nil {
		return err
	}

	backend := &state.LocalBackend{}
	st, err := backend.Load(projectDir)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()

	kc, err := kube.New(kubeconfigVal, ns)
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	pods, err := kc.GetPods(ctx, st.Name)
	if err != nil {
		return fmt.Errorf("get pods: %w", err)
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found for app %q", st.Name)
	}

	targetPods := pods
	if !logsAll {
		targetPods = pods[:1]
	}

	for _, pod := range targetPods {
		if len(targetPods) > 1 {
			fmt.Fprintf(cmd.OutOrStdout(), "=== %s ===\n", pod.Name)
		}

		stream, err := kc.GetPodLogs(ctx, pod.Name, logsFollow)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error getting logs for %s: %v\n", pod.Name, err)
			continue
		}

		io.Copy(cmd.OutOrStdout(), stream)
		stream.Close()
	}

	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/logs.go
git commit -m "feat: add vela logs command"
```

---

### Task 8: `vela delete` Command

**Files:**
- Create: `cmd/delete.go`

- [ ] **Step 1: Implement delete command**

```go
// cmd/delete.go
package cmd

import (
	"fmt"
	"os"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the application from the cluster",
	Args:  cobra.NoArgs,
	RunE:  runDelete,
}

func runDelete(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	projectDir, err := project.Find(cwd)
	if err != nil {
		return err
	}

	backend := &state.LocalBackend{}
	st, err := backend.Load(projectDir)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfigVal, ns)

	if hc.ReleaseExists(st.Name) {
		fmt.Fprintf(cmd.OutOrStdout(), "Uninstalling release %q...\n", st.Name)
		if err := hc.Uninstall(st.Name); err != nil {
			return fmt.Errorf("uninstall release: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Release %q uninstalled.\n", st.Name)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Release %q not found in cluster.\n", st.Name)
	}

	st.Status = state.StatusDeleted
	backend.Save(projectDir, st)

	fmt.Fprintf(cmd.OutOrStdout(), "App %q deleted.\n", st.Name)
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/delete.go
git commit -m "feat: add vela delete command with state sync"
```

---

### Task 9: `vela list` Command

**Files:**
- Create: `cmd/list.go`

- [ ] **Step 1: Implement list command**

```go
// cmd/list.go
package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all vela applications in the cluster",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfigVal, ns)

	releases, err := hc.List()
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	if len(releases) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No applications found in cluster.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tNAMESPACE\tSTATUS\tREVISION")
	for _, r := range releases {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", r.Name, r.Namespace, r.Status, r.Revision)
	}
	w.Flush()
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/list.go
git commit -m "feat: add vela list command for cluster-wide release query"
```

---

### Task 10: `vela configure` Command

**Files:**
- Create: `cmd/configure.go`

- [ ] **Step 1: Implement configure command with self-update**

Move the self-update logic from `cmd/update/update.go` into `cmd/configure.go`:

```go
// cmd/configure.go
package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var selfUpdate bool

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Manage vela settings and updates",
	RunE:  runConfigure,
}

func init() {
	configureCmd.Flags().BoolVar(&selfUpdate, "self-update", false, "update vela to the latest version")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	if selfUpdate {
		return runSelfUpdate(cmd)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "vela configure")
	fmt.Fprintln(cmd.OutOrStdout(), "No configurable settings yet.")
	fmt.Fprintln(cmd.OutOrStdout(), "\nUse --self-update to update vela to the latest version.")
	return nil
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

const repoAPI = "https://api.github.com/repos/khala-matrix/vela/releases/latest"

func runSelfUpdate(cmd *cobra.Command) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", Version)
	fmt.Fprintln(cmd.OutOrStdout(), "Checking for updates...")

	resp, err := http.Get(repoAPI)
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")
	if latest == current {
		fmt.Fprintln(cmd.OutOrStdout(), "Already up to date.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "New version available: %s → %s\n", Version, release.TagName)

	assetName := fmt.Sprintf("vela_%s_%s", runtime.GOOS, runtime.GOARCH)
	checksumName := "checksums.txt"

	var assetURL, checksumURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
		}
		if a.Name == checksumName {
			checksumURL = a.BrowserDownloadURL
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", assetName)
	binData, err := downloadFile(assetURL)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}

	if checksumURL != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "Verifying checksum...")
		checksumData, err := downloadFile(checksumURL)
		if err != nil {
			return fmt.Errorf("download checksums: %w", err)
		}
		if err := verifyFileChecksum(binData, checksumData, assetName); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable path: %w", err)
	}

	tmpFile := self + ".new"
	if err := os.WriteFile(tmpFile, binData, 0755); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}

	if err := os.Rename(tmpFile, self); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s successfully.\n", release.TagName)
	return nil
}

func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func verifyFileChecksum(data []byte, checksumFile []byte, name string) error {
	hash := sha256.Sum256(data)
	got := hex.EncodeToString(hash[:])

	for _, line := range strings.Split(string(checksumFile), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == name {
			if parts[0] == got {
				return nil
			}
			return fmt.Errorf("expected %s, got %s", parts[0], got)
		}
	}
	return fmt.Errorf("no checksum found for %s", name)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/configure.go
git commit -m "feat: add vela configure command with --self-update"
```

---

### Task 11: Rewire root.go and Delete Old Code

**Files:**
- Modify: `cmd/root.go`
- Delete: `cmd/app/` (entire directory — 8 files)
- Delete: `cmd/configure/` (entire directory — 1 file)
- Delete: `cmd/update/` (entire directory — 1 file)
- Delete: `pkg/store/` (entire directory — 2 files)
- Delete: `pkg/scaffold/templates/` (entire directory — 5 files)

- [ ] **Step 1: Rewrite root.go**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	kubeconfig string
	namespace  string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "vela",
	Short: "Deploy applications to k3s clusters",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	defaultKubeconfig := os.Getenv("KUBECONFIG")
	if defaultKubeconfig == "" {
		home, _ := os.UserHomeDir()
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
	}

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig, "path to kubeconfig file")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "default", "target namespace")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(versionCmd)
}
```

- [ ] **Step 2: Delete old directories**

```bash
rm -rf cmd/app/ cmd/configure/ cmd/update/ pkg/store/ pkg/scaffold/templates/
```

- [ ] **Step 3: Run go build to verify compilation**

Run: `go build -o vela main.go`
Expected: BUILD SUCCESS

- [ ] **Step 4: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass. `pkg/store` tests are gone (deleted). `pkg/scaffold` tests use new skeleton-based approach. `pkg/state` and `pkg/project` tests pass. `pkg/config`, `pkg/chart`, `pkg/helm`, `pkg/kube` tests are unchanged and pass.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: flatten commands, replace ~/.vela/ with project-local .vela/"
```

---

### Task 12: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md to reflect new architecture**

Update the Architecture and Key Conventions sections:

```markdown
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Vela — a Go CLI for generating Helm charts from `tech-stack.yaml` specs and deploying them to k3s clusters. Single binary, no runtime dependency on helm CLI. Designed for use as a CI pipeline tool and interactively from devbox development containers.

## Build & Test

\```bash
go build -o vela main.go        # build binary
go build ./...                   # check compilation
go test ./... -v                 # run all tests
go test ./pkg/state/ -v          # run single package tests
go test ./pkg/config/ -run TestParseTechStack_Full -v  # run single test
\```

## Architecture

CLI layer (`cmd/`) → business logic (`pkg/`). Flat command structure: `vela create`, `vela deploy`, `vela status`, etc. Each command is one file in `cmd/`.

Project state lives in `.vela/` directory (like `.git/`). Created by `vela create`, read/updated by all other commands.

- `pkg/config` — parses and validates `tech-stack.yaml` into `TechStack` struct, applies defaults
- `pkg/scaffold` — generates complete project skeletons from embedded `//go:embed` directory trees in `pkg/scaffold/skeletons/`
- `pkg/chart` — renders a complete Helm chart directory from a `TechStack`
- `pkg/state` — `Backend` interface + `LocalBackend` for `.vela/state.yaml` read/write
- `pkg/project` — `.vela/` directory detection (`Find`) and initialization (`Init`)
- `pkg/helm` — wraps Helm Go SDK for install/upgrade/uninstall/status/list
- `pkg/kube` — wraps client-go for Pod status queries and log streaming

Data flow: `vela create` → project skeleton + `.vela/state.yaml` → user builds images → `vela deploy` → `config.Parse()` → `chart.Generate()` → `.vela/chart/` → `helm.Install()` → k3s cluster → state synced to `.vela/state.yaml`.

## Key Conventions

- Skeleton template files live in `pkg/scaffold/skeletons/<template-id>/` and are embedded via `//go:embed all:skeletons`. Each `.tmpl` file is rendered with `text/template` and written with the `.tmpl` suffix stripped.
- Chart template files live in `pkg/chart/templates/` and are embedded via `//go:embed all:templates`. Helm template syntax is escaped in Go templates using `{{ "{{ .Values.x }}" }}`.
- `pkg/kube` has a `NewFromClientset()` constructor for tests using `k8s.io/client-go/kubernetes/fake`.
- Global flags (kubeconfig, namespace, verbose) are defined as PersistentFlags on the root command and accessed via `cmd.Flag("name").Value.String()` in subcommands.
- State backend is pluggable via the `state.Backend` interface. Only `LocalBackend` is implemented.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for refactored architecture"
```

---

### Task 13: End-to-End Verification

This is a manual verification task — not TDD. Run the full build and exercise each command.

- [ ] **Step 1: Build the binary**

Run: `go build -o vela main.go`
Expected: BUILD SUCCESS

- [ ] **Step 2: Run all tests**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 3: Test vela create (non-interactive)**

Run: `./vela create testapp --template nextjs-fastapi --registry registry.example.com/ns --domain example.com`
Expected: Creates `testapp/` with full skeleton + `.vela/state.yaml`

- [ ] **Step 4: Verify generated files**

Run: `ls -la testapp/ && cat testapp/.vela/state.yaml && cat testapp/tech-stack.yaml && cat testapp/frontend/next.config.ts`
Expected: All files exist with correct templated values

- [ ] **Step 5: Test vela version**

Run: `./vela version`
Expected: `vela dev (none)`

- [ ] **Step 6: Test vela list**

Run: `./vela list --kubeconfig sample/k3s/k3s-config`
Expected: Shows cluster releases (or empty list)

- [ ] **Step 7: Clean up test output**

Run: `rm -rf testapp/`

- [ ] **Step 8: Commit any fixes discovered during verification**

```bash
git add -A
git commit -m "fix: address issues found during end-to-end verification"
```

(Only if fixes were needed.)
