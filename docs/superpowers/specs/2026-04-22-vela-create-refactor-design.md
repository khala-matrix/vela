# Vela CLI Refactor: `vela create` + Flat Commands + Local State

## Goal

Refactor the Vela CLI so that `vela create` generates a complete, ready-to-build project skeleton (source code, Dockerfiles, build scripts, tech-stack.yaml, `.vela/` state directory) from an embedded template. Flatten all subcommands (`vela app create/deploy/status/...` → `vela create/deploy/status/...`). Introduce `.vela/` as a project-local state directory (like `.git/`), replacing the global `~/.vela/` store.

## Commands

| Command | Description |
|---------|-------------|
| `vela create` | TUI or `--flags`: select template, generate project skeleton + `.vela/` |
| `vela deploy` | Read `tech-stack.yaml` in cwd, generate chart to `.vela/chart/`, helm install/upgrade, update state |
| `vela status` | Show pod status for current project, sync state |
| `vela logs` | Stream logs for current project |
| `vela delete` | Helm uninstall, update state |
| `vela list` | Query cluster for all vela-managed releases (helm list + label filter) |
| `vela configure` | TUI for settings (state backend config) + `vela configure --self-update` |
| `vela version` | Print version |

### `vela create` behavior

**Interactive (TUI):**

```
$ vela create
⛵ vela create

Select a tech stack template:

> Next.js + FastAPI — Python backend, React frontend
  Static Site — Single container, nginx

Project name: myapp
Image registry: sgccr.ccs.tencentyun.com/mars-sandbox
Ingress domain: sandbox.cyber-psychosis.net

Review:
  Template: Next.js + FastAPI
  Project:  myapp
  Registry: sgccr.ccs.tencentyun.com/mars-sandbox
  Domain:   sandbox.cyber-psychosis.net

enter/y generate • n restart • esc back
```

Output:

```
Created project myapp/

  myapp/
  ├── .vela/state.yaml
  ├── tech-stack.yaml
  ├── build.sh
  ├── backend/
  │   ├── Dockerfile
  │   ├── main.py
  │   └── requirements.txt
  └── frontend/
      ├── Dockerfile
      ├── package.json
      ├── next.config.ts
      └── src/app/
          ├── layout.tsx
          ├── globals.css
          └── page.tsx

Next steps:
  cd myapp
  ./build.sh        # build & push images
  vela deploy       # deploy to cluster
```

**Non-interactive (CI):**

```
vela create myapp --template nextjs-fastapi --registry reg.example.com --domain example.com
```

Name as optional positional arg. When all required flags are provided, skip TUI.

### `vela deploy` behavior

1. Find `.vela/` in current directory (error if not found)
2. Read `tech-stack.yaml` → `config.Parse()`
3. Generate helm chart → `.vela/chart/`
4. Helm install (first deploy) or upgrade (subsequent)
5. Update `.vela/state.yaml` (status, revision, last_deployed, services)

Reads `--kubeconfig` flag or `KUBECONFIG` env, `--namespace` flag or defaults to `default`.

### `vela list` behavior

Query the cluster via `helm list` filtered by label `app.kubernetes.io/managed-by: vela`. No local registry needed. Shows name, namespace, status, revision, last deployed.

### `vela configure` behavior

Replaces both the old `vela configure` (template selection, now moved to `vela create`) and `vela update` (self-update).

- `vela configure` — TUI to manage settings (state backend, default namespace, etc.)
- `vela configure --self-update` — download latest binary from GitHub releases

Settings stored in `~/.vela/config.yaml` (global user config, separate from project `.vela/`).

## `.vela/` Directory

```
project/
├── .vela/
│   ├── state.yaml      # project state
│   └── chart/           # generated helm chart (gitignored)
├── tech-stack.yaml
└── ...project files
```

### state.yaml

```yaml
name: myapp
namespace: default
cluster: https://10.0.0.1:6443
last_deployed: "2026-04-22T10:30:00Z"
revision: 3
status: deployed
services:
  fastapi-backend:
    image: registry.example.com/api:latest
    ingress_path: /myapp/api
  nextjs-frontend:
    image: registry.example.com/web:latest
    ingress_path: /myapp
```

Fields:
- `name` — project name (from tech-stack.yaml)
- `namespace` — k8s namespace last deployed to
- `cluster` — cluster API server URL
- `last_deployed` — RFC3339 timestamp
- `revision` — helm release revision number
- `status` — one of: `created`, `deployed`, `failed`, `deleted`
- `services` — per-service snapshot (image, ingress path)

### State Backend Interface

```go
type Backend interface {
    Load(projectDir string) (*State, error)
    Save(projectDir string, state *State) error
}
```

1.0 implements `LocalBackend` only (reads/writes `.vela/state.yaml`). Interface allows future S3/HTTP backends. Backend selection configured in `~/.vela/config.yaml`:

```yaml
state:
  backend: local    # only option in 1.0
```

## Scaffold Architecture

### Approach: Embedded directory tree templates

Each template is a directory of `.tmpl` files under `pkg/scaffold/skeletons/<template-id>/`. The directory structure mirrors the output. At generation time, walk the tree, render each file through `text/template` with `Params`, write to destination.

```
pkg/scaffold/skeletons/
├── nextjs-fastapi/
│   ├── tech-stack.yaml.tmpl
│   ├── build.sh.tmpl
│   ├── backend/
│   │   ├── Dockerfile.tmpl
│   │   ├── main.py.tmpl
│   │   └── requirements.txt.tmpl
│   └── frontend/
│       ├── Dockerfile.tmpl
│       ├── package.json.tmpl
│       ├── tsconfig.json.tmpl          # static (no templating needed, still .tmpl for uniformity)
│       ├── postcss.config.mjs.tmpl
│       ├── next.config.ts.tmpl
│       └── src/app/
│           ├── layout.tsx.tmpl
│           ├── globals.css.tmpl
│           └── page.tsx.tmpl
└── static-site/
    ├── tech-stack.yaml.tmpl
    ├── build.sh.tmpl
    ├── Dockerfile.tmpl
    ├── nginx.conf.tmpl
    └── public/
        └── index.html.tmpl
```

### Template parameters

```go
type Params struct {
    Name     string  // project name (e.g. "myapp")
    Registry string  // image registry (e.g. "reg.example.com/ns")
    Domain   string  // ingress domain (e.g. "sandbox.example.com")
}
```

Same as current. Templates use `{{ .Name }}`, `{{ .Registry }}`, `{{ .Domain }}`.

### Scaffold Render function

```go
func RenderSkeleton(templateID string, params Params, outDir string) error
```

1. Walk `skeletons/<templateID>/` in the embedded FS
2. For each file, strip `.tmpl` suffix for output path
3. Create parent directories
4. Parse and execute template with `params`
5. Write to `outDir/<path>`

### Key template content decisions

**next.config.ts** — pre-configured with `basePath` and `rewrites`:

```typescript
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  basePath: "/{{ .Name }}",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.API_BACKEND_URL || "http://{{ .Name }}-backend:8000"}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
```

- `basePath` is set at build time to `/<project-name>` — matches the ingress path
- `rewrites` proxies `/api/*` to the backend k8s service at runtime via `API_BACKEND_URL` env var, with a sensible default using the k8s service DNS name
- Frontend code uses plain `fetch("/api/...")` — zero awareness of deployment paths

**tech-stack.yaml** — uses path-based ingress with global host:

```yaml
name: {{ .Name }}
ingress:
  host: {{ .Domain }}
services:
  - name: {{ .Name }}-backend
    image: {{ .Registry }}/{{ .Name }}-backend:latest
    port: 8000
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

**build.sh** — same pattern as current sample, parameterized:

```bash
#!/usr/bin/env bash
set -euo pipefail
REGISTRY="${REGISTRY:-{{ .Registry }}}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"
# ... registry auth, build backend, build frontend with NEXT_PUBLIC_BASE_PATH, push
```

**Frontend page.tsx** — minimal "deployed successfully" page:

```tsx
export default function Home() {
  return (
    <main>
      <h1>{{ .Name }}</h1>
      <p>Deployed with Vela</p>
    </main>
  );
}
```

**Backend main.py** — health endpoint + one demo endpoint:

```python
from fastapi import FastAPI
app = FastAPI()

@app.get("/api/health")
def health():
    return {"status": "ok"}

@app.get("/api/hello")
def hello():
    return {"message": "Hello from {{ .Name }}"}
```

## Package Structure Changes

### Before

```
cmd/
  app/          # parent + create/deploy/delete/status/update/logs/list
  configure/    # template → tech-stack.yaml
  update/       # self-update
  root.go
  version.go
pkg/
  scaffold/     # yaml template rendering
  store/        # ~/.vela/<app>/ management
  config/
  chart/
  helm/
  kube/
```

### After

```
cmd/
  create.go       # TUI + skeleton generation
  deploy.go       # chart gen + helm install/upgrade
  status.go       # pod status
  logs.go         # log streaming
  delete.go       # helm uninstall
  list.go         # cluster-wide query
  configure.go    # settings TUI + self-update
  root.go         # register all commands, global flags
  version.go
pkg/
  scaffold/       # skeleton rendering (replaces yaml-only templates)
    skeletons/    # embedded template trees
    scaffold.go   # RenderSkeleton + Template registry
  state/          # .vela/ state management
    state.go      # State struct, Backend interface
    local.go      # LocalBackend implementation
  project/        # .vela/ directory detection + initialization
    project.go    # FindProject (walk up), InitProject
  config/         # tech-stack.yaml parsing (unchanged)
  chart/          # helm chart generation (unchanged)
  helm/           # helm SDK wrapper (unchanged)
  kube/           # k8s client (unchanged)
```

### Deleted packages

- `pkg/store` — replaced by `pkg/state` + `pkg/project`
- `cmd/app/` — all subcommands moved to `cmd/*.go`
- `cmd/configure/` — merged into `cmd/configure.go`
- `cmd/update/` — merged into `cmd/configure.go`
- `pkg/scaffold/templates/` — replaced by `pkg/scaffold/skeletons/`

## Templates (1.0 Scope)

| ID | Skeleton files | Description |
|---|---|---|
| `nextjs-fastapi` | 12 files | Next.js + FastAPI, path-based ingress, API proxy |
| `static-site` | 5 files | Single nginx container |

Remaining templates (nextjs-go, react-springboot, vue-nestjs) deferred — same architecture, add later.

## Migration

- `sample/fastapi-nextjs/` stays as-is for reference/testing, but is no longer the canonical example — `vela create` generates the canonical skeleton
- Old `~/.vela/` global store is no longer used; existing users would need to re-deploy (acceptable at this stage)

## Error Handling

- `vela deploy/status/logs/delete` in a directory without `.vela/`: clear error "not a vela project — run 'vela create' first or cd into a project directory"
- `vela create` in a directory where `<name>/` already exists: error "directory already exists"
- State sync failures (cluster unreachable): update state.status to reflect last known state, warn but don't fail
