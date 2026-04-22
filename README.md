# Vela ⛵

A Go CLI for deploying containerized applications to k3s clusters. Single binary, no runtime dependencies.

## Install

**From GitHub Releases:**

```bash
# macOS (Apple Silicon)
curl -Lo vela https://github.com/khala-matrix/vela/releases/latest/download/vela_darwin_arm64
chmod +x vela && sudo mv vela /usr/local/bin/

# macOS (Intel)
curl -Lo vela https://github.com/khala-matrix/vela/releases/latest/download/vela_darwin_amd64
chmod +x vela && sudo mv vela /usr/local/bin/

# Linux (amd64)
curl -Lo vela https://github.com/khala-matrix/vela/releases/latest/download/vela_linux_amd64
chmod +x vela && sudo mv vela /usr/local/bin/
```

**From source:**

```bash
go install -ldflags "-X github.com/mars/vela/cmd.Version=dev" github.com/mars/vela@latest
```

**Self-update:**

```bash
vela update
```

## Prerequisites

### Kubeconfig

Vela needs access to a k3s (or any Kubernetes) cluster. Provide the kubeconfig in one of three ways (in order of precedence):

```bash
# 1. CLI flag
vela app deploy myapp --kubeconfig /path/to/kubeconfig

# 2. Environment variable
export KUBECONFIG=/path/to/kubeconfig
vela app deploy myapp

# 3. Default path (~/.kube/config)
vela app deploy myapp
```

### Container Registry

Images must be pushed to a registry accessible from the cluster. Vela itself does not handle image builds or pushes — use your existing CI pipeline or the `build.sh` pattern shown in the sample project.

**Docker Hub:**

```bash
docker login -u <username> --password-stdin <<< "<password>"
```

**JFrog Artifactory (using Personal Access Token):**

```bash
# Set in CI environment variables
export REGISTRY_USER=<email-or-username>
export REGISTRY_PASSWORD=<jfrog-access-token>

# Login
echo "${REGISTRY_PASSWORD}" | docker login <your-jfrog-instance>.jfrog.io -u "${REGISTRY_USER}" --password-stdin
```

For CI pipelines, configure `REGISTRY_USER` and `REGISTRY_PASSWORD` as pipeline secrets. The sample `build.sh` script automatically handles login when these variables are set:

```bash
export REGISTRY_USER=ci-bot
export REGISTRY_PASSWORD=$JFROG_ACCESS_TOKEN
export REGISTRY=<your-jfrog-instance>.jfrog.io/<repo>
./build.sh
```

**Tencent Cloud Container Registry:**

```bash
export REGISTRY_USER=<username>
export REGISTRY_PASSWORD=<password>
echo "${REGISTRY_PASSWORD}" | docker login sgccr.ccs.tencentyun.com -u "${REGISTRY_USER}" --password-stdin
```

## Quick Start

```bash
# Generate a tech-stack.yaml from a template (interactive TUI)
vela configure

# Or write your own tech-stack.yaml (see examples below)

# Create app — generates Helm chart locally
vela app create myapp -f tech-stack.yaml

# Deploy to cluster
vela app deploy myapp

# Check status
vela app status myapp

# View logs
vela app logs myapp --all

# Update after config change
vela app update myapp -f tech-stack.yaml

# Delete
vela app delete myapp
```

## Tech Stack Templates

Run `vela configure` to interactively select a template and fill in parameters. Available templates:

| Template | Frontend | Backend | Use Case |
|---|---|---|---|
| **Next.js + FastAPI** | Next.js (React) | Python FastAPI | AI/data apps, full-stack web |
| **Next.js + Go** | Next.js (React) | Go (Gin/Echo) | High-performance APIs |
| **React + Spring Boot** | React (Vite) | Java Spring Boot | Enterprise applications |
| **Vue + NestJS** | Vue 3 | Node.js NestJS | TypeScript full-stack |
| **Static Site** | — | — | Hugo, Astro, docs sites |

### Example: Next.js + FastAPI

```yaml
name: my-project
services:
  - name: my-project-backend
    image: registry.example.com/my-project-backend:latest
    port: 8000
    replicas: 1
    env:
      - name: CORS_ORIGINS
        value: "*"
      - name: PYTHONUNBUFFERED
        value: "1"
    resources:
      cpu: 250m
      memory: 256Mi
    ingress:
      enabled: true
      host: my-project-api.example.com
  - name: my-project-frontend
    image: registry.example.com/my-project-frontend:latest
    port: 3000
    replicas: 1
    resources:
      cpu: 250m
      memory: 256Mi
    ingress:
      enabled: true
      host: my-project.example.com
```

### Example: Single Service (Legacy Format)

The legacy `app:` format is still supported and auto-converts internally:

```yaml
app:
  name: simple-api
  image: registry.example.com/simple-api:v1.0.0
  port: 8080
  replicas: 2
  env:
    - name: APP_ENV
      value: production
  resources:
    cpu: 500m
    memory: 512Mi
```

### Example: With Dependencies

Add managed databases as Bitnami sub-charts:

```yaml
name: my-project
services:
  - name: api
    image: registry.example.com/api:latest
    port: 8080
    replicas: 1
    resources:
      cpu: 250m
      memory: 256Mi
dependencies:
  postgresql:
    version: "16"
    storage: 5Gi
    database: mydb
  redis:
    version: "7.0"
```

Supported dependencies: `mysql`, `postgresql`, `redis`, `mongodb`.

## tech-stack.yaml Reference

```yaml
name: <project-name>           # Required for multi-service

services:                       # List of services
  - name: <service-name>        # Required — unique per project
    image: <image:tag>          # Required
    port: <container-port>      # Required
    replicas: 1                 # Default: 1
    env:                        # Optional
      - name: KEY
        value: "value"
    resources:                  # Default: cpu=250m, memory=256Mi
      cpu: 250m
      memory: 256Mi
    ingress:                    # Optional
      enabled: true
      host: app.example.com    # Auto-generated if omitted
      path: /                  # Default: /

dependencies:                   # Optional — Bitnami sub-charts
  <mysql|postgresql|redis|mongodb>:
    version: "<version>"        # Required
    storage: 1Gi                # Default: 1Gi
    database: <db-name>         # Default: project name
    password: <password>        # Optional
```

## Releasing

Vela uses GoReleaser for cross-platform builds. To create a release:

```bash
git tag v0.1.0
git push origin v0.1.0
# GitHub Actions builds and publishes to Releases
```

Users update with:

```bash
vela update
```
