# Vela

A Go CLI for deploying containerized applications to k3s clusters. Single binary, no runtime dependencies.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/khala-matrix/vela/main/bootstrap.sh | bash
```

This downloads the latest binary to `~/.vela/bin/`, adds it to your PATH, and installs the Claude Code skill for agent-driven infrastructure management.

**Manual install:**

```bash
# macOS (Apple Silicon)
curl -Lo vela https://github.com/khala-matrix/vela/releases/latest/download/vela_darwin_arm64
chmod +x vela && sudo mv vela /usr/local/bin/

# macOS (Intel)
curl -Lo vela https://github.com/khala-matrix/vela/releases/latest/download/vela_darwin_amd64

# Linux (amd64)
curl -Lo vela https://github.com/khala-matrix/vela/releases/latest/download/vela_linux_amd64
```

**Self-update:**

```bash
vela configure --self-update
```

## Quick Start

```bash
# Create a project from a template
vela create my-app -t nextjs-fastapi-pg

# Build and push images
cd my-app
./build.sh

# Deploy to cluster
vela deploy

# Check status
vela status

# View database credentials
vela credentials
```

## Commands

| Command | Purpose |
|---------|---------|
| `vela create <name> -t <template>` | Scaffold a new project |
| `vela deploy` | Deploy or upgrade to cluster |
| `vela status` | Show deployment status |
| `vela list` | List all deployed apps |
| `vela delete` | Remove app from cluster |
| `vela logs` | Stream pod logs |
| `vela credentials` | Show database credentials |
| `vela guide <template>` | Show setup guide for a template |
| `vela configure --self-update` | Update vela to latest version |

All commands support `--output json` (`-o json`) for machine-parseable output.

## Templates

| ID | Stack | Database |
|----|-------|----------|
| `nextjs-fastapi` | Next.js + FastAPI | - |
| `nextjs-fastapi-pg` | Next.js + FastAPI + PostgreSQL | PostgreSQL |
| `static-site` | Nginx static files | - |

Run `vela guide <template>` to see a complete setup reference with `tech-stack.yaml` examples, Dockerfiles, and path coordination rules.

## tech-stack.yaml

Every project has a `tech-stack.yaml` that describes services, ingress, and dependencies:

```yaml
name: my-app
ingress:
  host: devbox.example.com
services:
  - name: my-app-backend
    image: registry.example.com/my-app-backend:latest
    port: 8000
    env:
      - name: DATABASE_URL
        value: "postgresql+asyncpg://postgres:secret@my-app-postgresql:5432/my-app"
    ingress:
      enabled: true
      path: /sandbox/my-app/api
      stripPrefix: false
  - name: my-app-frontend
    image: registry.example.com/my-app-frontend:latest
    port: 3000
    ingress:
      enabled: true
      path: /sandbox/my-app
      stripPrefix: false
dependencies:
  postgresql:
    version: "16"
    database: my-app
    password: "secret"
    imageRegistry: registry.example.com/tools
```

Supported dependencies: `mysql`, `postgresql`, `redis`, `mongodb`.

## Claude Code Integration

Vela ships with a Claude Code skill for agent-driven infrastructure management. The bootstrap installer sets it up automatically.

To install the skill manually:

```bash
mkdir -p ~/.claude/skills/vela-infra
cp skills/vela-infra.md ~/.claude/skills/vela-infra/SKILL.md
```

The skill teaches Claude Code how to use all vela commands with `--output json`, debug deployment issues, and handle path coordination.

## Releasing

Push a version tag to trigger an automated release via GitHub Actions + GoReleaser:

```bash
git tag v0.x.x
git push origin v0.x.x
```
