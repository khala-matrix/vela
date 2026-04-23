# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Vela — a Go CLI for generating Helm charts from `tech-stack.yaml` specs and deploying them to k3s clusters. Single binary, no runtime dependency on helm CLI. Designed for use as a CI pipeline tool and interactively from devbox development containers. Default target is a corporate k3s cluster behind an LB with Traefik ingress controller.

## Build & Test

```bash
go build -o vela main.go        # build binary
go build ./...                   # check compilation
go test ./... -v                 # run all tests
go test ./pkg/state/ -v          # run single package tests
go test ./pkg/config/ -run TestParseTechStack_Full -v  # run single test
```

## Architecture

CLI layer (`cmd/`) → business logic (`pkg/`). Flat command structure: `vela create`, `vela deploy`, `vela status`, etc. Each command is one file in `cmd/`.

Project state lives in `.vela/` directory (like `.git/`). Created by `vela create`, read/updated by all other commands.

- `pkg/config` — parses and validates `tech-stack.yaml` into `TechStack` struct, applies defaults
- `pkg/scaffold` — generates complete project skeletons from embedded `//go:embed` directory trees in `pkg/scaffold/skeletons/`
- `pkg/chart` — renders a complete Helm chart directory from a `TechStack`
- `pkg/state` — `Backend` interface + `LocalBackend` for `.vela/state.yaml` read/write
- `pkg/project` — `.vela/` directory detection (`Find`) and initialization (`Init`)
- `pkg/helm` — wraps Helm Go SDK for install/upgrade/uninstall/status/list
- `pkg/kube` — wraps client-go for Pod status queries, log streaming, and resource queries (ingresses, services)

Data flow: `vela create` → project skeleton + `.vela/state.yaml` → user builds images → `vela deploy` → `config.Parse()` → `chart.Generate()` → `.vela/chart/` → `helm.Install()` → k3s cluster → state synced to `.vela/state.yaml`.

## Key Conventions

- Skeleton template files live in `pkg/scaffold/skeletons/<template-id>/` and are embedded via `//go:embed all:skeletons`. Each `.tmpl` file is rendered with `text/template` and written with the `.tmpl` suffix stripped.
- Chart template files live in `pkg/chart/templates/` and are embedded via `//go:embed all:templates`. Helm template syntax is escaped in Go templates using `{{ "{{ .Values.x }}" }}`.
- `pkg/kube` has a `NewFromClientset()` constructor for tests using `k8s.io/client-go/kubernetes/fake`.
- Global flags (kubeconfig, namespace, verbose, insecure) are defined as PersistentFlags on the root command and accessed via `cmd.Flag("name").Value.String()` in subcommands. Default namespace is `sandbox`.
- `--insecure` flag skips TLS certificate verification for k3s clusters with self-signed certs. Implemented via custom `insecureGetter` in `pkg/helm` that wraps the full RESTClientGetter interface.
- State backend is pluggable via the `state.Backend` interface. Only `LocalBackend` is implemented.
- Scaffold `Params` includes `Name`, `Namespace`, `Registry`, `Domain`, `BaseRegistry`. Namespace is used as ingress path prefix (`/namespace/appname`) to support LB routing.
- Ingress path coordination: frontend uses Next.js `basePath: "/ns/app"`, backend uses FastAPI `APIRouter(prefix="/ns/app/api")`, no stripPrefix — Traefik passes the full path through.
- Dockerfile templates use `{{ .BaseRegistry }}` for corporate base image registries. Multi-stage builds use `USER root` for build stage, `USER 1000` for runtime.
- Environment defaults are loaded from `.env` file (see `.env.example`). Configurable via `VELA_REGISTRY`, `VELA_DOMAIN`, `VELA_BASE_REGISTRY`, `VELA_DB_IMAGE_REGISTRY`.
