# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Vela — a Go CLI for generating Helm charts from `tech-stack.yaml` specs and deploying them to k3s clusters. Single binary, no runtime dependency on helm CLI. Designed for use as a CI pipeline skill and interactively from devbox development containers.

## Build & Test

```bash
go build -o vela main.go        # build binary
go build ./...                   # check compilation
go test ./... -v                 # run all tests
go test ./pkg/store/ -v          # run single package tests
go test ./pkg/config/ -run TestParseTechStack_Full -v  # run single test
```

## Architecture

CLI layer (`cmd/`) → business logic (`pkg/`). Cobra commands in `cmd/app/` are thin wrappers that compose `pkg/` packages.

- `pkg/config` — parses and validates `tech-stack.yaml` into `TechStack` struct, applies defaults (replicas=1, cpu=250m, memory=256Mi, storage=1Gi)
- `pkg/chart` — renders a complete Helm chart directory from a `TechStack` using `//go:embed` templates
- `pkg/chart/dependencies.go` — registry mapping service types (mysql/postgresql/redis/mongodb) to Bitnami chart names, repos, and default values. Adding a new dependency type = one new entry here.
- `pkg/helm` — wraps Helm Go SDK (`helm.sh/helm/v3`) for install/upgrade/uninstall/status/list
- `pkg/kube` — wraps client-go for Pod status queries and log streaming
- `pkg/store` — manages `~/.vela/<app>/` local storage (chart files + `meta.json`)

Data flow: `tech-stack.yaml` → `config.Parse()` → `chart.Generate()` → `~/.vela/<name>/chart/` → `helm.Install()` → k3s cluster.

## Key Conventions

- Chart template files live in `pkg/chart/templates/` and are embedded via `//go:embed all:templates`. Helm template syntax is escaped in Go templates using `{{ "{{ .Values.x }}" }}`.
- `pkg/kube` has a `NewFromClientset()` constructor for tests using `k8s.io/client-go/kubernetes/fake`.
- Helm client tests are limited to construction/compilation since they require a live cluster.
- Global flags (kubeconfig, namespace, verbose) are defined as PersistentFlags on the root command and accessed via `cmd.Flag("name").Value.String()` in subcommands.
