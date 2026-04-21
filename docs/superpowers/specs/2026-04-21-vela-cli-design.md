# Vela CLI — Design Spec

A Golang CLI tool for deploying containerized applications to k3s clusters from devbox development containers. Named after the Vela (船帆座) constellation.

## Core Decisions

- **Stack**: Go + Cobra + client-go + Helm Go SDK (`helm.sh/helm/v3`)
- **Single binary**: No runtime dependency on helm CLI or other tools
- **Helm chart generation**: Renders standard Helm charts from a `tech-stack.yaml` spec
- **Dependency services**: Uses Bitnami community Helm charts as sub-charts
- **Image-only**: Assumes container images are pre-built by CI; vela does not build images
- **Two-step deploy**: `create` generates chart, `deploy` installs to cluster (separate commands)
- **Local storage**: `~/.vela/<app-name>/` stores generated charts and app metadata
- **Kubeconfig**: Default `~/.kube/config`, overridable via `--kubeconfig` flag or `KUBECONFIG` env var

## CLI Command Structure

```
vela
├── app
│   ├── create <name> -f <tech-stack.yaml>   # Generate Helm chart to ~/.vela/<name>/
│   ├── deploy <name>                         # helm install to k3s
│   ├── update <name> -f <tech-stack.yaml>     # re-render chart + helm upgrade
│   ├── delete <name>                         # helm uninstall + clean local files
│   ├── list                                  # List all apps (local + cluster status)
│   ├── status <name>                         # Release status + Pod status
│   └── logs <name>                           # Stream application Pod logs
```

Global flags:
- `--kubeconfig` — kubeconfig path, default `~/.kube/config`, supports `KUBECONFIG` env
- `--namespace` — target namespace, default `default`
- `--verbose` — verbose output

## tech-stack.yaml Format

```yaml
app:
  name: myapp
  image: registry.example.com/myapp:v1.0.0
  port: 8080
  replicas: 1                                    # optional, default 1
  env:                                           # optional
    - name: APP_ENV
      value: development
    - name: DB_HOST
      value: "{{ .Release.Name }}-mysql"         # Helm template vars supported
  resources:                                     # optional, has defaults
    cpu: 500m
    memory: 512Mi

dependencies:
  mysql:
    version: "8.0"
    storage: 5Gi                                 # optional, default 1Gi
    password: mypassword                         # optional, default auto-generated
    database: myapp_db                           # optional, default app.name
  redis:
    version: "7.0"                               # minimal form, rest uses defaults
```

Design notes:
- `app` section: what image to run and how
- `dependencies`: map of service type → config. Key is the service type (mysql/redis/mongodb/postgresql), vela maps each to the corresponding Bitnami chart with sensible defaults
- `env` values may use Helm template syntax to reference dependency services — vela preserves these during chart rendering

## Project Architecture

```
vela/
├── cmd/                          # Cobra command definitions
│   ├── root.go                   # Root command + global flags
│   └── app/
│       ├── create.go
│       ├── deploy.go
│       ├── update.go
│       ├── delete.go
│       ├── list.go
│       ├── status.go
│       └── logs.go
├── pkg/
│   ├── config/                   # tech-stack.yaml parsing & validation
│   │   └── techstack.go
│   ├── chart/                    # Helm chart generation
│   │   ├── generator.go          # Render Helm chart from TechStack struct
│   │   ├── templates/            # go:embed Helm template files
│   │   │   ├── Chart.yaml.tmpl
│   │   │   ├── values.yaml.tmpl
│   │   │   └── templates/
│   │   │       ├── deployment.yaml.tmpl
│   │   │       ├── service.yaml.tmpl
│   │   │       └── _helpers.tpl.tmpl
│   │   └── dependencies.go       # Service type → Bitnami chart mapping
│   ├── helm/                     # Helm SDK wrapper
│   │   └── client.go             # install/upgrade/uninstall/status/list
│   ├── kube/                     # client-go wrapper
│   │   └── client.go             # Pod logs, status queries
│   └── store/                    # ~/.vela/ local storage management
│       └── store.go              # App metadata read/write
├── main.go
├── go.mod
└── go.sum
```

## Data Flow

1. **create**: `tech-stack.yaml` → `pkg/config` parse & validate → `pkg/chart` render → write to `~/.vela/<name>/`
2. **deploy**: `pkg/store` read local chart → `pkg/helm` install to cluster
3. **update**: re-parse `tech-stack.yaml` → `pkg/chart` re-render → `pkg/helm` upgrade (requires `-f` flag to point to updated yaml)
4. **delete**: `pkg/helm` uninstall → `pkg/store` clean local files
5. **list**: `pkg/store` list local apps + `pkg/helm` query cluster release status
6. **status**: `pkg/helm` query release + `pkg/kube` query Pod details
7. **logs**: `pkg/kube` stream Pod logs via client-go

Key technical details:
- Chart template files embedded in binary via `//go:embed`
- `pkg/chart/dependencies.go` maintains a registry: `mysql → bitnami/mysql`, `redis → bitnami/redis`, etc., with default values per service type
- Helm SDK and client-go share the same kubeconfig / REST config

## Error Handling

**create:**
- tech-stack.yaml missing or malformed → error with format hint
- App name already exists in `~/.vela/` → error, suggest delete first or use update
- Unsupported dependency type → error listing supported types

**deploy:**
- App not created → error, prompt to create first
- Cluster unreachable → error showing kubeconfig path and connection failure
- Release already exists → suggest using update instead

**delete:**
- App doesn't exist locally → error
- Release exists in cluster but no local record → still attempt helm uninstall

**logs/status:**
- Pod not started → show current state (Pending/CrashLoopBackOff), not empty output
- Multiple Pods → default to first Pod, `--all` flag to show all

## MVP Supported Dependencies

- mysql
- postgresql
- redis
- mongodb

Adding a new type requires only a new entry in `dependencies.go` (chart name, repo URL, default values).

## Usage Context

Vela is designed to be used as a skill in CI pipelines — after a CI build succeeds, vela commands are invoked via CLI to deploy the application to k3s. It is also usable interactively from devbox development containers.
