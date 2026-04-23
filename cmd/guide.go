package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/mars/vela/pkg/scaffold"
	"github.com/spf13/cobra"
)

var (
	guideName      string
	guideNamespace string
	guideRegistry  string
	guideDomain    string
	guideBuilder   string
)

var guideCmd = &cobra.Command{
	Use:   "guide [template-id]",
	Short: "Show setup guide and examples for a template",
	Long: `Output a markdown guide with tech-stack.yaml examples, build.sh,
Dockerfile tips, and path coordination instructions for the given template.

Available templates:
  nextjs-fastapi        Next.js + FastAPI
  nextjs-fastapi-pg     Next.js + FastAPI + PostgreSQL
  static-site           Static Site (nginx)`,
	Args: cobra.ExactArgs(1),
	RunE: runGuide,
}

func init() {
	guideCmd.Flags().StringVar(&guideName, "name", "my-app", "project name used in examples")
	guideCmd.Flags().StringVar(&guideNamespace, "namespace", "sandbox", "namespace used in examples")
	guideCmd.Flags().StringVar(&guideRegistry, "registry", defaultRegistry, "image registry used in examples")
	guideCmd.Flags().StringVar(&guideDomain, "domain", defaultDomain, "ingress domain used in examples")
	guideCmd.Flags().StringVar(&guideBuilder, "builder", "docker", "container build tool used in examples (docker, buildah)")
}

type guideData struct {
	Name            string
	Namespace       string
	Registry        string
	Domain          string
	BaseRegistry    string
	DBImageRegistry string
	BuildTool       string
	BuildCmd        string
}

func runGuide(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	var found bool
	for _, t := range scaffold.Templates {
		if t.ID == templateID {
			found = true
			break
		}
	}
	if !found {
		ids := make([]string, len(scaffold.Templates))
		for i, t := range scaffold.Templates {
			ids[i] = t.ID
		}
		return fmt.Errorf("unknown template %q, available: %s", templateID, strings.Join(ids, ", "))
	}

	guideTmpl, ok := guideTemplates[templateID]
	if !ok {
		return fmt.Errorf("no guide available for template %q", templateID)
	}

	buildCmd := "build"
	if guideBuilder == "buildah" {
		buildCmd = "bud"
	}

	data := guideData{
		Name:            guideName,
		Namespace:       guideNamespace,
		Registry:        guideRegistry,
		Domain:          guideDomain,
		BaseRegistry:    defaultBaseRegistry,
		DBImageRegistry: defaultDBImageRegistry,
		BuildTool:       guideBuilder,
		BuildCmd:        buildCmd,
	}

	tmpl, err := template.New("guide").Parse(guideTmpl)
	if err != nil {
		return fmt.Errorf("parse guide template: %w", err)
	}

	if isJSON() {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("render guide: %w", err)
		}
		var tmplObj *scaffold.Template
		for i := range scaffold.Templates {
			if scaffold.Templates[i].ID == templateID {
				tmplObj = &scaffold.Templates[i]
				break
			}
		}
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"template":    templateID,
			"title":       tmplObj.Name,
			"description": tmplObj.Description,
			"params":      data,
			"guide":       buf.String(),
		})
	}

	return tmpl.Execute(cmd.OutOrStdout(), data)
}

var guideTemplates = map[string]string{
	"nextjs-fastapi-pg": `# Next.js + FastAPI + PostgreSQL — Setup Guide

> All examples below use project **{{ .Name }}** in namespace **{{ .Namespace }}**.
> Customize with: vela guide nextjs-fastapi-pg --name <name> --namespace <ns>

## Project Structure

` + "```" + `
{{ .Name }}/
├── backend/
│   ├── Dockerfile
│   ├── main.py              # FastAPI app
│   └── requirements.txt
├── frontend/
│   ├── Dockerfile
│   ├── next.config.ts        # basePath + rewrites
│   ├── package.json
│   └── src/app/
│       └── page.tsx          # fetch() calls with path prefix
├── tech-stack.yaml           # vela deployment spec
└── build.sh                  # build & push images
` + "```" + `

## tech-stack.yaml

` + "```yaml" + `
name: {{ .Name }}
ingress:
  host: {{ .Domain }}
services:
  - name: {{ .Name }}-backend
    image: {{ .Registry }}/{{ .Name }}-backend:latest
    port: 8000
    env:
      - name: DATABASE_URL
        value: "postgresql+asyncpg://postgres:<password>@{{ .Name }}-postgresql:5432/{{ .Name }}"
      - name: CORS_ORIGINS
        value: "*"
      - name: PYTHONUNBUFFERED
        value: "1"
    ingress:
      enabled: true
      path: /{{ .Namespace }}/{{ .Name }}/api
      stripPrefix: false
  - name: {{ .Name }}-frontend
    image: {{ .Registry }}/{{ .Name }}-frontend:latest
    port: 3000
    ingress:
      enabled: true
      path: /{{ .Namespace }}/{{ .Name }}
      stripPrefix: false
dependencies:
  postgresql:
    version: "16"
    database: {{ .Name }}
    password: "<password>"
    imageRegistry: {{ .DBImageRegistry }}
` + "```" + `

## build.sh

` + "```bash" + `
#!/usr/bin/env bash
set -euo pipefail

REGISTRY="${REGISTRY:-{{ .Registry }}}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"

{{ .BuildTool }} {{ .BuildCmd }} --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}-backend:${TAG}" ./backend
{{ .BuildTool }} {{ .BuildCmd }} --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}-frontend:${TAG}" ./frontend
{{ .BuildTool }} push "${REGISTRY}/{{ .Name }}-backend:${TAG}"
{{ .BuildTool }} push "${REGISTRY}/{{ .Name }}-frontend:${TAG}"
` + "```" + `

## Path Coordination (Critical)

Four places must agree on the path prefix **` + "`" + `/{{ .Namespace }}/{{ .Name }}` + "`" + `**:

### 1. Backend — FastAPI route prefix

` + "```python" + `
router = APIRouter(prefix="/{{ .Namespace }}/{{ .Name }}/api")
# All routes (e.g. /health, /todos) are served under /{{ .Namespace }}/{{ .Name }}/api/...
` + "```" + `

### 2. Frontend — next.config.ts

` + "```ts" + `
const nextConfig: NextConfig = {
  output: "standalone",
  basePath: "/{{ .Namespace }}/{{ .Name }}",       // static assets, routing
  rewrites: async () => [
    {
      source: "/api/:path*",                       // relative to basePath
      destination: "http://{{ .Name }}-{{ .Name }}-backend:8000/{{ .Namespace }}/{{ .Name }}/api/:path*",
    },
  ],
};
` + "```" + `

### 3. Frontend — fetch() calls

**basePath does NOT apply to fetch().** You must add the prefix manually:

` + "```tsx" + `
// WRONG — will request /api/health (404)
fetch("/api/health")

// CORRECT — requests /{{ .Namespace }}/{{ .Name }}/api/health
const BASE = "/{{ .Namespace }}/{{ .Name }}";
fetch(` + "`" + `${BASE}/api/health` + "`" + `)
fetch(` + "`" + `${BASE}/api/todos` + "`" + `)
` + "```" + `

### 4. Ingress — tech-stack.yaml (shown above)

` + "```" + `
Backend ingress path:  /{{ .Namespace }}/{{ .Name }}/api   → backend:8000
Frontend ingress path: /{{ .Namespace }}/{{ .Name }}       → frontend:3000
` + "```" + `

Traefik passes the full path through (stripPrefix: false), so the backend
receives the complete path including the prefix.

## Dockerfile Tips

**Backend:**
` + "```dockerfile" + `
FROM {{ .BaseRegistry }}/python:3.12-slim-bookworm
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
USER 1000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
` + "```" + `

**Frontend (multi-stage):**
` + "```dockerfile" + `
FROM {{ .BaseRegistry }}/node:22-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM {{ .BaseRegistry }}/node:22-alpine
WORKDIR /app
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
USER 1000
CMD ["node", "server.js"]
` + "```" + `

## Database

PostgreSQL is deployed as a native K8s Deployment (not a Helm subchart).
The database password is auto-generated by ` + "`" + `vela create` + "`" + ` and stored in
` + "`" + `.vela/state.yaml` + "`" + `. Use ` + "`" + `vela credentials` + "`" + ` to view it.

The K8s Secret ` + "`" + `{{ .Name }}-postgresql` + "`" + ` contains POSTGRES_PASSWORD and DATABASE_URL.
The backend reads DATABASE_URL from its environment (set in tech-stack.yaml).

## Quick Start

` + "```bash" + `
vela create {{ .Name }} -t nextjs-fastapi-pg --namespace {{ .Namespace }}
cd {{ .Name }}
./build.sh
vela deploy
vela credentials   # show database connection details
# Visit: https://{{ .Domain }}/{{ .Namespace }}/{{ .Name }}
` + "```" + `
`,

	"nextjs-fastapi": `# Next.js + FastAPI — Setup Guide

> All examples below use project **{{ .Name }}** in namespace **{{ .Namespace }}**.
> Customize with: vela guide nextjs-fastapi --name <name> --namespace <ns>

## Project Structure

` + "```" + `
{{ .Name }}/
├── backend/
│   ├── Dockerfile
│   ├── main.py
│   └── requirements.txt
├── frontend/
│   ├── Dockerfile
│   ├── next.config.ts
│   ├── package.json
│   └── src/app/
│       ├── layout.tsx
│       └── page.tsx
├── tech-stack.yaml
└── build.sh
` + "```" + `

## tech-stack.yaml

` + "```yaml" + `
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
    ingress:
      enabled: true
      path: /{{ .Namespace }}/{{ .Name }}/api
      stripPrefix: false
  - name: {{ .Name }}-frontend
    image: {{ .Registry }}/{{ .Name }}-frontend:latest
    port: 3000
    ingress:
      enabled: true
      path: /{{ .Namespace }}/{{ .Name }}
      stripPrefix: false
` + "```" + `

## build.sh

` + "```bash" + `
#!/usr/bin/env bash
set -euo pipefail

REGISTRY="${REGISTRY:-{{ .Registry }}}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"

{{ .BuildTool }} {{ .BuildCmd }} --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}-backend:${TAG}" ./backend
{{ .BuildTool }} {{ .BuildCmd }} --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}-frontend:${TAG}" ./frontend
{{ .BuildTool }} push "${REGISTRY}/{{ .Name }}-backend:${TAG}"
{{ .BuildTool }} push "${REGISTRY}/{{ .Name }}-frontend:${TAG}"
` + "```" + `

## Path Coordination (Critical)

Four places must agree on the path prefix **` + "`" + `/{{ .Namespace }}/{{ .Name }}` + "`" + `**:

### 1. Backend — FastAPI route prefix

` + "```python" + `
router = APIRouter(prefix="/{{ .Namespace }}/{{ .Name }}/api")
` + "```" + `

### 2. Frontend — next.config.ts

` + "```ts" + `
const nextConfig: NextConfig = {
  output: "standalone",
  basePath: "/{{ .Namespace }}/{{ .Name }}",
  rewrites: async () => [
    {
      source: "/api/:path*",
      destination: "http://{{ .Name }}-{{ .Name }}-backend:8000/{{ .Namespace }}/{{ .Name }}/api/:path*",
    },
  ],
};
` + "```" + `

### 3. Frontend — fetch() calls

**basePath does NOT apply to fetch().** Add the prefix manually:

` + "```tsx" + `
const BASE = "/{{ .Namespace }}/{{ .Name }}";
fetch(` + "`" + `${BASE}/api/health` + "`" + `)
` + "```" + `

### 4. Ingress (in tech-stack.yaml)

` + "```" + `
Backend:  /{{ .Namespace }}/{{ .Name }}/api → backend:8000
Frontend: /{{ .Namespace }}/{{ .Name }}     → frontend:3000
` + "```" + `

## Quick Start

` + "```bash" + `
vela create {{ .Name }} -t nextjs-fastapi --namespace {{ .Namespace }}
cd {{ .Name }}
./build.sh
vela deploy
# Visit: https://{{ .Domain }}/{{ .Namespace }}/{{ .Name }}
` + "```" + `
`,

	"static-site": `# Static Site — Setup Guide

> All examples below use project **{{ .Name }}** in namespace **{{ .Namespace }}**.
> Customize with: vela guide static-site --name <name> --namespace <ns>

## Project Structure

` + "```" + `
{{ .Name }}/
├── Dockerfile
├── nginx.conf
├── public/
│   └── index.html
├── tech-stack.yaml
└── build.sh
` + "```" + `

## tech-stack.yaml

` + "```yaml" + `
name: {{ .Name }}
ingress:
  host: {{ .Domain }}
services:
  - name: {{ .Name }}
    image: {{ .Registry }}/{{ .Name }}:latest
    port: 80
    ingress:
      enabled: true
      path: /{{ .Namespace }}/{{ .Name }}
` + "```" + `

## build.sh

` + "```bash" + `
#!/usr/bin/env bash
set -euo pipefail

REGISTRY="${REGISTRY:-{{ .Registry }}}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"

{{ .BuildTool }} {{ .BuildCmd }} --platform "${PLATFORM}" -t "${REGISTRY}/{{ .Name }}:${TAG}" .
{{ .BuildTool }} push "${REGISTRY}/{{ .Name }}:${TAG}"
` + "```" + `

## Quick Start

` + "```bash" + `
vela create {{ .Name }} -t static-site --namespace {{ .Namespace }}
cd {{ .Name }}
# Edit public/index.html
./build.sh
vela deploy
# Visit: https://{{ .Domain }}/{{ .Namespace }}/{{ .Name }}
` + "```" + `
`,
}
