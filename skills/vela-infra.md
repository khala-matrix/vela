---
name: vela-infra
description: Use when the user asks to create, deploy, manage, debug, or check status of applications on the k3s cluster. Also use when they mention vela, deploy, infrastructure, cluster operations, or need to scaffold a new project.
---

# Vela Infrastructure Management

You are managing infrastructure using the `vela` CLI tool. **Always use `--output json` (`-o json`)** when running vela commands so you can parse results reliably.

## Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `vela create <name> -t <template>` | Scaffold a new project | `-t`, `--registry`, `--namespace`, `--base-registry`, `-o json` |
| `vela deploy` | Deploy or upgrade to cluster | `--kubeconfig`, `--namespace`, `--insecure`, `-o json` |
| `vela status` | Check deployment status | `-o json` |
| `vela list` | List all deployed apps | `-o json` |
| `vela credentials` | Show database credentials | `-o json` |
| `vela delete` | Remove app from cluster | `-o json` |
| `vela logs` | Stream pod logs | `--follow`, `--tail` |
| `vela guide <template>` | Show setup guide for a template | `--name`, `--namespace`, `-o json` |

## Templates

| ID | Stack | Has DB |
|----|-------|--------|
| `nextjs-fastapi` | Next.js + FastAPI | No |
| `nextjs-fastapi-pg` | Next.js + FastAPI + PostgreSQL | Yes |
| `static-site` | Nginx static files | No |

## Standard Workflow

### Create a new project
```bash
vela create my-app -t nextjs-fastapi-pg --namespace sandbox -o json
```

### Build and deploy
```bash
cd my-app
./build.sh
vela deploy -o json
vela status -o json
```

### Check cluster state
```bash
vela list -o json      # all apps
vela status -o json    # current project
```

### Database credentials
```bash
vela credentials -o json
```
Only available for `nextjs-fastapi-pg` template projects.

### Get setup guidance for manual projects
```bash
vela guide nextjs-fastapi-pg --name my-app --namespace sandbox
```
Outputs a complete reference with tech-stack.yaml, Dockerfiles, path coordination rules.

## JSON Output Format

All commands with `-o json` return structured JSON. Parse the output to determine success/failure and extract data.

**`vela create -o json`** returns:
```json
{"name": "my-app", "template": "nextjs-fastapi-pg", "directory": "my-app", "status": "created"}
```

**`vela deploy -o json`** returns:
```json
{"name": "my-app", "namespace": "sandbox", "status": "deployed", "revision": 2, "action": "upgraded", "ingresses": [...]}
```

**`vela status -o json`** returns:
```json
{"name": "my-app", "namespace": "sandbox", "status": "deployed", "revision": 2, "pods": [...], "services": [...], "ingresses": [...]}
```
When not deployed: `{"name": "my-app", "status": "not_deployed"}`

**`vela list -o json`** returns an array of apps (same structure as status).

**`vela credentials -o json`** returns:
```json
{"postgresql": {"host": "my-app-postgresql", "port": 5432, "database": "my-app", "user": "postgres", "password": "..."}}
```

**`vela delete -o json`** returns:
```json
{"name": "my-app", "status": "deleted"}
```

## Path Coordination

This is the most common source of bugs. When deploying Next.js + FastAPI apps, **four places must agree** on the path prefix `/<namespace>/<app-name>`:

1. **Backend** — FastAPI `APIRouter(prefix="/<ns>/<name>/api")`
2. **Frontend next.config.ts** — `basePath: "/<ns>/<name>"` + rewrites to backend
3. **Frontend fetch() calls** — Must manually prefix: `fetch(\`/${ns}/${name}/api/health\`)`
   - `basePath` does NOT apply to `fetch()` — this is the #1 gotcha
4. **Ingress** in tech-stack.yaml — backend path `/<ns>/<name>/api`, frontend path `/<ns>/<name>`

When debugging path issues, use `vela guide <template> --name <name> --namespace <ns>` to see the correct configuration.

## Debugging

When deployment has issues:

1. **Check pod status**: `vela status -o json` — look at `pods[].status` and `pods[].ready`
2. **Check logs**: `vela logs` — look for crash loops, connection errors, import errors
3. **Frontend loads but API fails**:
   - Check browser network tab — URL should include `/<namespace>/<name>/api/...`
   - If URL is just `/api/...`, the frontend fetch calls are missing the basePath prefix
4. **Nothing loads (404)**:
   - Check ingress paths match the backend/frontend configuration
   - Verify the app is actually deployed: `vela status -o json`
5. **Database connection fails**:
   - Check if postgresql pod is running: look for `*-postgresql` in pod list
   - Verify DATABASE_URL in tech-stack.yaml matches the service name
   - Use `vela credentials -o json` to get the actual password

## Environment Defaults

Defaults are loaded from `.env` file in the project root (see `.env.example`). The following environment variables configure vela:

- `VELA_REGISTRY` — image registry (default: `registry.example.com/myteam`)
- `VELA_DOMAIN` — ingress domain (default: `apps.example.com`)
- `VELA_BASE_REGISTRY` — base image registry for Dockerfiles
- `VELA_DB_IMAGE_REGISTRY` — image registry for database containers
- Namespace: `sandbox` (via `--namespace` flag)

## Important Rules

- Always use `-o json` for programmatic access
- `vela create` with `-o json` requires `<name>` and `--template` (no interactive TUI)
- `vela deploy` must be run from within the project directory (where `.vela/` exists)
- After modifying source code, **rebuild images** with `./build.sh` before `vela deploy`
- Image builds require `docker` and access to the registry
- The kubeconfig must be specified via `--kubeconfig` flag or `KUBECONFIG` env var
- Use `--insecure` when connecting to clusters with self-signed TLS certificates
