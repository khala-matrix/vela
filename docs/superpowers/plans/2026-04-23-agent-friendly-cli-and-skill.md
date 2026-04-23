# Agent-Friendly CLI + Claude Code Skill — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make vela CLI machine-parseable via `--output json` on key commands, harden non-interactive mode, then wrap everything in a Claude Code skill that agents can invoke to manage infrastructure end-to-end.

**Architecture:** Two layers — (1) CLI changes: add JSON output format, non-interactive safety, and structured errors; (2) A Claude Code skill (SKILL.md + reference docs) installed as a project-local skill in `.claude/skills/vela-infra/`.

**Tech Stack:** Go (CLI), Markdown (skill definition)

---

### Task 1: Add shared `--output` flag and JSON helper

**Files:**
- Modify: `cmd/root.go`
- Create: `cmd/output.go`

- [ ] **Step 1: Create `cmd/output.go`**

Create `cmd/output.go` with a shared output format flag and JSON helper:

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
)

var outputFormat string

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func isJSON() bool {
	return outputFormat == "json"
}
```

- [ ] **Step 2: Register flag on root command**

In `cmd/root.go`, inside `func init()`, add after the `insecure` flag:

```go
rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "output format (text, json)")
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/output.go cmd/root.go
git commit -m "feat: add --output flag and JSON helper for agent-friendly output"
```

---

### Task 2: JSON output for `vela status`

**Files:**
- Modify: `cmd/status.go`

- [ ] **Step 1: Define JSON response struct and add JSON branch**

In `cmd/status.go`, add a struct for the JSON response and modify `runStatus` to branch on `isJSON()`. The JSON struct:

```go
type statusOutput struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Status    string            `json:"status"`
	Revision  int               `json:"revision"`
	Updated   string            `json:"updated,omitempty"`
	Pods      []podOutput       `json:"pods"`
	Services  []serviceOutput   `json:"services,omitempty"`
	Ingresses []ingressOutput   `json:"ingresses,omitempty"`
}

type podOutput struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Ready  bool   `json:"ready"`
}

type serviceOutput struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
}

type ingressOutput struct {
	Name string `json:"name"`
	Host string `json:"host,omitempty"`
	Path string `json:"path,omitempty"`
	URL  string `json:"url"`
}
```

At the end of `runStatus`, after all data is collected, if `isJSON()`, build `statusOutput` from the collected `rel`, `pods`, `services`, `ingresses` and call `writeJSON(cmd.OutOrStdout(), out)` then return. Otherwise keep existing text output.

When release not found and `isJSON()`, output:
```go
writeJSON(cmd.OutOrStdout(), map[string]any{
    "name":   st.Name,
    "status": "not_deployed",
})
```

- [ ] **Step 2: Run existing tests**

Run: `go test ./... -v`
Expected: All PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/status.go
git commit -m "feat: vela status --output json for structured status output"
```

---

### Task 3: JSON output for `vela list`

**Files:**
- Modify: `cmd/list.go`

- [ ] **Step 1: Add JSON branch to `runList`**

Define a struct:

```go
type listAppOutput struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	Status    string          `json:"status"`
	Revision  int             `json:"revision"`
	Pods      []podOutput     `json:"pods,omitempty"`
	Services  []serviceOutput `json:"services,omitempty"`
	Ingresses []ingressOutput `json:"ingresses,omitempty"`
}
```

Reuse `podOutput`, `serviceOutput`, `ingressOutput` from `cmd/status.go` (they are in the same package).

If `isJSON()`, build `[]listAppOutput` from the releases and kube data, then `writeJSON`. When empty: output `[]` (empty JSON array).

- [ ] **Step 2: Verify compilation and test**

Run: `go build ./... && go test ./...`
Expected: All PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/list.go
git commit -m "feat: vela list --output json for structured app listing"
```

---

### Task 4: JSON output for `vela credentials`

**Files:**
- Modify: `cmd/credentials.go`

- [ ] **Step 1: Add JSON branch**

If `isJSON()`, output the `s.Credentials` map directly via `writeJSON`. The existing `state.Credential` struct already has proper yaml tags — add json tags to it:

In `pkg/state/state.go`, update `Credential` struct to have both yaml and json tags:

```go
type Credential struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Database string `yaml:"database" json:"database"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
}
```

Also add json tags to `State`, `ServiceState` (for future use):

```go
type State struct {
	Name         string                  `yaml:"name" json:"name"`
	Namespace    string                  `yaml:"namespace" json:"namespace"`
	Cluster      string                  `yaml:"cluster,omitempty" json:"cluster,omitempty"`
	LastDeployed string                  `yaml:"last_deployed,omitempty" json:"last_deployed,omitempty"`
	Revision     int                     `yaml:"revision,omitempty" json:"revision,omitempty"`
	Status       string                  `yaml:"status" json:"status"`
	Services     map[string]ServiceState `yaml:"services,omitempty" json:"services,omitempty"`
	Credentials  map[string]*Credential  `yaml:"credentials,omitempty" json:"credentials,omitempty"`
}

type ServiceState struct {
	Image       string `yaml:"image" json:"image"`
	IngressPath string `yaml:"ingress_path,omitempty" json:"ingress_path,omitempty"`
}
```

Then in `cmd/credentials.go`, before the existing for-loop output:

```go
if isJSON() {
    return writeJSON(cmd.OutOrStdout(), s.Credentials)
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./... -v`
Expected: All PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/credentials.go pkg/state/state.go
git commit -m "feat: vela credentials --output json, add json tags to state structs"
```

---

### Task 5: JSON output for `vela deploy`

**Files:**
- Modify: `cmd/deploy.go`

- [ ] **Step 1: Add JSON output at end of deploy**

Define a struct:

```go
type deployOutput struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	Status    string          `json:"status"`
	Revision  int             `json:"revision"`
	Action    string          `json:"action"` // "installed" or "upgraded"
	Ingresses []ingressOutput `json:"ingresses,omitempty"`
}
```

Track whether the action was install or upgrade with a local var `action := "installed"` / `action = "upgraded"`. At the end of `runDeploy`, if `isJSON()`, build `deployOutput` and write it. Otherwise keep existing text.

- [ ] **Step 2: Verify**

Run: `go build ./... && go test ./...`

- [ ] **Step 3: Commit**

```bash
git add cmd/deploy.go
git commit -m "feat: vela deploy --output json for structured deploy result"
```

---

### Task 6: JSON output for `vela delete` and `vela create`

**Files:**
- Modify: `cmd/delete.go`
- Modify: `cmd/create.go`

- [ ] **Step 1: JSON output for delete**

If `isJSON()`, output:
```go
writeJSON(cmd.OutOrStdout(), map[string]any{
    "name":   st.Name,
    "status": "deleted",
})
```

- [ ] **Step 2: JSON output for create (non-interactive path only)**

In `generateProject`, if `isJSON()`, output:
```go
type createOutput struct {
    Name      string `json:"name"`
    Template  string `json:"template"`
    Directory string `json:"directory"`
    Status    string `json:"status"` // "created"
}
```

Keep existing text output in else branch.

- [ ] **Step 3: Non-interactive safety for create**

In `runCreate`, when `name == "" || createTemplate == ""` and `isJSON()`, return an error instead of launching TUI:

```go
if isJSON() {
    return fmt.Errorf("--name and --template are required when using --output json (non-interactive mode)")
}
```

- [ ] **Step 4: Verify**

Run: `go build ./... && go test ./...`

- [ ] **Step 5: Commit**

```bash
git add cmd/delete.go cmd/create.go
git commit -m "feat: vela delete/create --output json, block TUI in json mode"
```

---

### Task 7: JSON output for `vela guide`

**Files:**
- Modify: `cmd/guide.go`

- [ ] **Step 1: Add JSON mode**

When `isJSON()`, output a structured object instead of raw markdown:

```go
type guideOutput struct {
	Template    string            `json:"template"`
	Title       string            `json:"title"`
	Params      guideData         `json:"params"`
	Guide       string            `json:"guide"` // rendered markdown as a single string
}
```

This allows an agent to get the rendered markdown as a string field while also having structured metadata. Render the template into a `bytes.Buffer`, then wrap in the struct.

- [ ] **Step 2: Verify**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add cmd/guide.go
git commit -m "feat: vela guide --output json wraps markdown in structured envelope"
```

---

### Task 8: Create the Claude Code skill — SKILL.md

**Files:**
- Create: `.claude/skills/vela-infra/SKILL.md`

- [ ] **Step 1: Create skill directory**

```bash
mkdir -p .claude/skills/vela-infra
```

- [ ] **Step 2: Write SKILL.md**

The skill file must have YAML frontmatter with `name` and `description`, then the skill content. This is the core document that Claude Code loads when the skill is invoked.

Create `.claude/skills/vela-infra/SKILL.md`:

```markdown
---
name: vela-infra
description: Use when the user asks to create, deploy, manage, debug, or check status of applications on the k3s cluster. Also use when they mention vela, deploy, infrastructure, or cluster operations.
---

# Vela Infrastructure Management

You are managing infrastructure using the `vela` CLI tool. All commands support `--output json` for structured output — **always use `--output json`** when running vela commands so you can parse results reliably.

## Available Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `vela create <name> -t <template>` | Scaffold a new project | `--template`, `--registry`, `--namespace`, `--base-registry` |
| `vela deploy` | Deploy/upgrade to cluster | `--kubeconfig`, `--namespace`, `--insecure` |
| `vela status` | Check deployment status | `--output json` |
| `vela list` | List all deployed apps | `--output json` |
| `vela credentials` | Show database credentials | `--output json` |
| `vela delete` | Remove app from cluster | |
| `vela logs` | Stream pod logs | `--follow`, `--tail` |
| `vela guide <template>` | Show setup guide | `--name`, `--namespace` |

## Templates

| ID | Stack | Has DB |
|----|-------|--------|
| `nextjs-fastapi` | Next.js + FastAPI | No |
| `nextjs-fastapi-pg` | Next.js + FastAPI + PostgreSQL | Yes |
| `static-site` | Nginx static files | No |

## Standard Workflow

### Creating a new project
```bash
vela create my-app -t nextjs-fastapi-pg --namespace sandbox --output json
```

### Building and deploying
```bash
cd my-app
./build.sh                          # build & push container images
vela deploy --output json           # deploy to cluster
vela status --output json           # verify deployment
```

### Checking status
```bash
vela status --output json           # current project
vela list --output json             # all apps in cluster
```

### Getting database credentials
```bash
vela credentials --output json      # returns host, port, user, password
```

## Path Coordination Rules

This is the most common source of bugs. Four places must agree on the path prefix `/<namespace>/<app-name>`:

1. **Backend** — FastAPI `APIRouter(prefix="/<ns>/<name>/api")`
2. **Frontend next.config.ts** — `basePath: "/<ns>/<name>"` + rewrites
3. **Frontend fetch() calls** — Must manually prefix all fetch URLs with `/<ns>/<name>` (basePath does NOT apply to fetch)
4. **Ingress** — Backend path `/<ns>/<name>/api`, frontend path `/<ns>/<name>`

Use `vela guide <template> --name <name> --namespace <ns>` to generate a complete reference with correct paths.

## Debugging Checklist

When something doesn't work after deploy:

1. `vela status --output json` — check pod status, are all pods Ready?
2. `vela logs` — check for crash loops, connection errors
3. If frontend loads but API fails:
   - Check browser network tab — is the request URL correct? (should include namespace prefix)
   - Check backend logs for 404s
   - Verify path coordination (see above)
4. If nothing loads:
   - Check ingress rules: paths must match
   - Check if backend pod is CrashLoopBackOff (usually DB connection or import error)

## Environment Defaults

- Registry: configured via `VELA_REGISTRY` env var
- Domain: configured via `VELA_DOMAIN` env var
- Base registry: configured via `VELA_BASE_REGISTRY` env var
- Namespace: `sandbox`
- kubeconfig: use `--kubeconfig` flag or KUBECONFIG env var

## Important Notes

- Always use `--output json` when running vela commands programmatically
- `vela create` with `--output json` requires `--template` flag (won't launch interactive TUI)
- `vela deploy` must be run from within the project directory (where `.vela/` exists)
- After modifying source code, you must rebuild images with `./build.sh` before `vela deploy`
- `vela credentials` only works for projects created with the `nextjs-fastapi-pg` template
```

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/vela-infra/SKILL.md
git commit -m "feat: add vela-infra Claude Code skill for agent-driven infrastructure management"
```

---

### Task 9: Full test suite and smoke test

**Files:**
- None (verification only)

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -v
```
Expected: All PASS.

- [ ] **Step 2: Build binary**

```bash
go build -o vela main.go
```

- [ ] **Step 3: Smoke test JSON output**

```bash
./vela create smoketest -t nextjs-fastapi-pg --namespace sandbox --output json
cat smoketest/.vela/state.yaml
cd smoketest && ../vela credentials --output json
cd ..
rm -rf smoketest
```

Expected: JSON output for create and credentials.

- [ ] **Step 4: Smoke test guide JSON**

```bash
./vela guide nextjs-fastapi-pg --output json | head -5
```

Expected: JSON envelope with guide field.

- [ ] **Step 5: Verify skill file loads**

```bash
cat .claude/skills/vela-infra/SKILL.md | head -5
```

Expected: YAML frontmatter with name and description.

- [ ] **Step 6: Clean up**

```bash
rm -f vela
```

- [ ] **Step 7: Commit if any fixes needed, otherwise done**
