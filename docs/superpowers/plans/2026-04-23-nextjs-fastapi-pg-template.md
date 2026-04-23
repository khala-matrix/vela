# Next.js + FastAPI + PostgreSQL Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `nextjs-fastapi-pg` scaffold template with Todo CRUD app, native PostgreSQL K8s deployment, auto-generated credentials, and `vela credentials` command.

**Architecture:** New skeleton directory generates FastAPI+SQLAlchemy backend, Next.js Todo frontend, and tech-stack.yaml with postgresql dependency. Chart generator renders native K8s resources (Deployment/Service/PVC/Secret) for PostgreSQL instead of Bitnami subchart. Password auto-generated at create time, stored in state, viewable via `vela credentials`.

**Tech Stack:** Go, text/template, Cobra, Helm Go SDK, FastAPI, SQLAlchemy async, asyncpg, Next.js, Tailwind CSS

---

## File Map

**Create:**
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/tech-stack.yaml.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/build.sh.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/Dockerfile.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/main.py.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/requirements.txt.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/Dockerfile.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/next.config.ts.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/package.json.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/tsconfig.json.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/postcss.config.mjs.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/globals.css.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/layout.tsx.tmpl`
- `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/page.tsx.tmpl`
- `pkg/chart/templates/templates/database.yaml.tmpl`
- `cmd/credentials.go`

**Modify:**
- `pkg/scaffold/scaffold.go` — add `DBPassword` to `Params`, register new template
- `pkg/config/techstack.go` — add `ImageRegistry` to `Dependency`
- `pkg/state/state.go` — add `Credentials` to `State`
- `pkg/chart/generator.go` — add database chart data, render `database.yaml.tmpl`
- `pkg/chart/templates/values.yaml.tmpl` — add database values section
- `pkg/chart/templates/templates/deployment.yaml.tmpl` — add `envFrom` support
- `cmd/create.go` — generate DB password, pass to scaffold, save credentials to state
- `cmd/root.go` — register `credentialsCmd`

---

### Task 1: Add `ImageRegistry` to Dependency struct and `DBPassword` to Scaffold Params

**Files:**
- Modify: `pkg/config/techstack.go:76-81`
- Modify: `pkg/scaffold/scaffold.go:23-28`
- Modify: `pkg/config/techstack_test.go`
- Modify: `pkg/scaffold/scaffold_test.go`

- [ ] **Step 1: Add `ImageRegistry` field to `Dependency` struct**

In `pkg/config/techstack.go`, change:

```go
type Dependency struct {
	Version  string `yaml:"version"`
	Storage  string `yaml:"storage,omitempty"`
	Password string `yaml:"password,omitempty"`
	Database string `yaml:"database,omitempty"`
}
```

to:

```go
type Dependency struct {
	Version       string `yaml:"version"`
	Storage       string `yaml:"storage,omitempty"`
	Password      string `yaml:"password,omitempty"`
	Database      string `yaml:"database,omitempty"`
	ImageRegistry string `yaml:"imageRegistry,omitempty"`
}
```

- [ ] **Step 2: Add `DBPassword` field to scaffold `Params`**

In `pkg/scaffold/scaffold.go`, change:

```go
type Params struct {
	Name         string
	Namespace    string
	Registry     string
	Domain       string
	BaseRegistry string
}
```

to:

```go
type Params struct {
	Name         string
	Namespace    string
	Registry     string
	Domain       string
	BaseRegistry string
	DBPassword   string
}
```

- [ ] **Step 3: Register new template**

In `pkg/scaffold/scaffold.go`, add to `Templates` slice after the static-site entry:

```go
{ID: "nextjs-fastapi-pg", Name: "Next.js + FastAPI + PostgreSQL", Description: "Full-stack web app with database — Python backend, React frontend, PostgreSQL"},
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/config/ ./pkg/scaffold/ -v`
Expected: All existing tests PASS (struct changes are backward-compatible).

- [ ] **Step 5: Commit**

```bash
git add pkg/config/techstack.go pkg/scaffold/scaffold.go
git commit -m "feat: add ImageRegistry to Dependency, DBPassword to Params, register nextjs-fastapi-pg template"
```

---

### Task 2: Add `Credentials` to State struct

**Files:**
- Modify: `pkg/state/state.go`
- Modify: `pkg/state/state_test.go`

- [ ] **Step 1: Add `Credential` type and `Credentials` field to `State`**

In `pkg/state/state.go`, add after `ServiceState`:

```go
type Credential struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}
```

And add field to `State`:

```go
type State struct {
	Name         string                  `yaml:"name"`
	Namespace    string                  `yaml:"namespace"`
	Cluster      string                  `yaml:"cluster,omitempty"`
	LastDeployed string                  `yaml:"last_deployed,omitempty"`
	Revision     int                     `yaml:"revision,omitempty"`
	Status       string                  `yaml:"status"`
	Services     map[string]ServiceState `yaml:"services,omitempty"`
	Credentials  map[string]*Credential  `yaml:"credentials,omitempty"`
}
```

- [ ] **Step 2: Add test for state with credentials**

In `pkg/state/state_test.go`, add:

```go
func TestState_WithCredentials(t *testing.T) {
	dir := t.TempDir()
	b := &LocalBackend{}

	s := &State{
		Name:      "myapp",
		Namespace: "sandbox",
		Status:    StatusCreated,
		Credentials: map[string]*Credential{
			"postgresql": {
				Host:     "myapp-postgresql",
				Port:     5432,
				Database: "myapp",
				User:     "postgres",
				Password: "testpass123",
			},
		},
	}

	if err := b.Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := b.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cred, ok := loaded.Credentials["postgresql"]
	if !ok {
		t.Fatal("postgresql credential missing")
	}
	if cred.Host != "myapp-postgresql" {
		t.Errorf("expected host myapp-postgresql, got %s", cred.Host)
	}
	if cred.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cred.Port)
	}
	if cred.Password != "testpass123" {
		t.Errorf("expected password testpass123, got %s", cred.Password)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/state/ -v`
Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/state/state.go pkg/state/state_test.go
git commit -m "feat: add Credentials to State for database password storage"
```

---

### Task 3: Create `vela credentials` command

**Files:**
- Create: `cmd/credentials.go`
- Modify: `cmd/root.go:49`

- [ ] **Step 1: Create `cmd/credentials.go`**

```go
package cmd

import (
	"fmt"

	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var credentialsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Show database credentials for the current project",
	RunE:  runCredentials,
}

func runCredentials(cmd *cobra.Command, args []string) error {
	projectDir, err := project.Find(".")
	if err != nil {
		return err
	}

	b := &state.LocalBackend{}
	s, err := b.Load(projectDir)
	if err != nil {
		return err
	}

	if len(s.Credentials) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No credentials configured.")
		return nil
	}

	for name, cred := range s.Credentials {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", capitalize(name))
		fmt.Fprintf(cmd.OutOrStdout(), "  Host:     %s:%d\n", cred.Host, cred.Port)
		fmt.Fprintf(cmd.OutOrStdout(), "  Database: %s\n", cred.Database)
		fmt.Fprintf(cmd.OutOrStdout(), "  User:     %s\n", cred.User)
		fmt.Fprintf(cmd.OutOrStdout(), "  Password: %s\n", cred.Password)
		fmt.Fprintf(cmd.OutOrStdout(), "  URL:      postgresql://%s:%s@%s:%d/%s\n",
			cred.User, cred.Password, cred.Host, cred.Port, cred.Database)
	}
	return nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
```

- [ ] **Step 2: Register command in `cmd/root.go`**

Add after the `rootCmd.AddCommand(versionCmd)` line:

```go
rootCmd.AddCommand(credentialsCmd)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/credentials.go cmd/root.go
git commit -m "feat: add vela credentials command"
```

---

### Task 4: Wire DB password generation into `vela create`

**Files:**
- Modify: `cmd/create.go:62-99`

- [ ] **Step 1: Add password generation import and helper**

At the top of `cmd/create.go`, add `"crypto/rand"` and `"encoding/hex"` to imports. Add helper function:

```go
func generatePassword() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)[:16]
}
```

- [ ] **Step 2: Update `generateProject` to generate password and save credentials**

Change the `generateProject` function. The current function signature is:

```go
func generateProject(cmd *cobra.Command, name, templateID, registry, domain, baseRegistry string) error {
```

In the function body, after `ns := cmd.Flag("namespace").Value.String()`, add password generation logic. Before the `scaffold.RenderSkeleton` call, generate the password and set it in params. After `project.Init`, save credentials to state if the template has a DB dependency.

Replace the full function body from `ns := ...` through the end:

```go
	ns := cmd.Flag("namespace").Value.String()

	dbPassword := ""
	if templateID == "nextjs-fastapi-pg" {
		dbPassword = generatePassword()
	}

	params := scaffold.Params{
		Name:         name,
		Namespace:    ns,
		Registry:     registry,
		Domain:       domain,
		BaseRegistry: baseRegistry,
		DBPassword:   dbPassword,
	}

	if err := scaffold.RenderSkeleton(templateID, params, outDir); err != nil {
		return fmt.Errorf("generate skeleton: %w", err)
	}

	if err := project.Init(outDir, name, ns); err != nil {
		os.RemoveAll(outDir)
		return fmt.Errorf("init project: %w", err)
	}

	if dbPassword != "" {
		b := &state.LocalBackend{}
		s, err := b.Load(outDir)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}
		s.Credentials = map[string]*state.Credential{
			"postgresql": {
				Host:     name + "-postgresql",
				Port:     5432,
				Database: name,
				User:     "postgres",
				Password: dbPassword,
			},
		}
		if err := b.Save(outDir, s); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created project %s/\n\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "Next steps:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  cd %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  ./build.sh        # build & push images\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  vela deploy       # deploy to cluster\n")
	if dbPassword != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  vela credentials  # show database credentials\n")
	}
	return nil
```

- [ ] **Step 3: Add state import**

Add `"github.com/mars/vela/pkg/state"` and `"crypto/rand"` and `"encoding/hex"` to the imports block at the top of `cmd/create.go`.

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/create.go
git commit -m "feat: auto-generate DB password in vela create, save to state"
```

---

### Task 5: Create backend skeleton files

**Files:**
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/requirements.txt.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/Dockerfile.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/main.py.tmpl`

- [ ] **Step 1: Create `requirements.txt.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/requirements.txt.tmpl`:

```
fastapi==0.115.12
uvicorn==0.34.2
sqlalchemy[asyncio]==2.0.40
asyncpg==0.30.0
```

- [ ] **Step 2: Create `Dockerfile.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/Dockerfile.tmpl`:

```dockerfile
FROM {{ if .BaseRegistry }}{{ .BaseRegistry }}/{{ end }}python:3.12-slim-bookworm
WORKDIR /app
COPY --chown=1000:1000 requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY --chown=1000:1000 . .
EXPOSE 8000
USER 1000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

- [ ] **Step 3: Create `main.py.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/main.py.tmpl`:

```python
from contextlib import asynccontextmanager
from datetime import datetime, timezone
from typing import Optional

from fastapi import FastAPI, APIRouter, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from sqlalchemy import Boolean, Column, DateTime, Integer, String, text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase
import os


DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql+asyncpg://postgres:{{ .DBPassword }}@{{ .Name }}-postgresql:5432/{{ .Name }}",
)

engine = create_async_engine(DATABASE_URL)
async_session = async_sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)


class Base(DeclarativeBase):
    pass


class Todo(Base):
    __tablename__ = "todos"
    id = Column(Integer, primary_key=True, autoincrement=True)
    title = Column(String(255), nullable=False)
    completed = Column(Boolean, default=False)
    created_at = Column(DateTime, default=lambda: datetime.now(timezone.utc))


@asynccontextmanager
async def lifespan(app: FastAPI):
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    yield
    await engine.dispose()


app = FastAPI(title="{{ .Name }} API", lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=os.getenv("CORS_ORIGINS", "*").split(","),
    allow_methods=["*"],
    allow_headers=["*"],
)

router = APIRouter(prefix="/{{ .Namespace }}/{{ .Name }}/api")


@router.get("/health")
def health():
    return {"status": "ok"}


@router.get("/db-health")
async def db_health():
    try:
        async with async_session() as session:
            await session.execute(text("SELECT 1"))
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "detail": str(e)}


class TodoCreate(BaseModel):
    title: str


class TodoUpdate(BaseModel):
    title: Optional[str] = None
    completed: Optional[bool] = None


class TodoResponse(BaseModel):
    id: int
    title: str
    completed: bool
    created_at: datetime

    class Config:
        from_attributes = True


@router.get("/todos", response_model=list[TodoResponse])
async def list_todos():
    async with async_session() as session:
        result = await session.execute(
            text("SELECT id, title, completed, created_at FROM todos ORDER BY created_at DESC")
        )
        rows = result.fetchall()
        return [
            TodoResponse(id=r.id, title=r.title, completed=r.completed, created_at=r.created_at)
            for r in rows
        ]


@router.post("/todos", response_model=TodoResponse, status_code=201)
async def create_todo(body: TodoCreate):
    async with async_session() as session:
        todo = Todo(title=body.title)
        session.add(todo)
        await session.commit()
        await session.refresh(todo)
        return TodoResponse(
            id=todo.id, title=todo.title, completed=todo.completed, created_at=todo.created_at
        )


@router.patch("/todos/{todo_id}", response_model=TodoResponse)
async def update_todo(todo_id: int, body: TodoUpdate):
    async with async_session() as session:
        todo = await session.get(Todo, todo_id)
        if not todo:
            raise HTTPException(status_code=404, detail="Todo not found")
        if body.title is not None:
            todo.title = body.title
        if body.completed is not None:
            todo.completed = body.completed
        await session.commit()
        await session.refresh(todo)
        return TodoResponse(
            id=todo.id, title=todo.title, completed=todo.completed, created_at=todo.created_at
        )


@router.delete("/todos/{todo_id}")
async def delete_todo(todo_id: int):
    async with async_session() as session:
        todo = await session.get(Todo, todo_id)
        if not todo:
            raise HTTPException(status_code=404, detail="Todo not found")
        await session.delete(todo)
        await session.commit()
        return {"ok": True}


app.include_router(router)
```

- [ ] **Step 4: Verify compilation** (template must be embeddable)

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add pkg/scaffold/skeletons/nextjs-fastapi-pg/backend/
git commit -m "feat: add nextjs-fastapi-pg backend skeleton (FastAPI + SQLAlchemy)"
```

---

### Task 6: Create frontend skeleton files

**Files:**
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/Dockerfile.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/next.config.ts.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/package.json.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/tsconfig.json.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/postcss.config.mjs.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/globals.css.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/layout.tsx.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/page.tsx.tmpl`

- [ ] **Step 1: Create `Dockerfile.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/Dockerfile.tmpl`:

```dockerfile
FROM {{ if .BaseRegistry }}{{ .BaseRegistry }}/{{ end }}node:22-alpine AS deps
WORKDIR /app
COPY package.json ./
RUN npm install

FROM {{ if .BaseRegistry }}{{ .BaseRegistry }}/{{ end }}node:22-alpine AS builder
USER root
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM {{ if .BaseRegistry }}{{ .BaseRegistry }}/{{ end }}node:22-alpine
WORKDIR /app
COPY --from=builder --chown=1000:1000 /app/.next/standalone ./
COPY --from=builder --chown=1000:1000 /app/.next/static ./.next/static

EXPOSE 3000
ENV HOSTNAME="0.0.0.0"
USER 1000
CMD ["node", "server.js"]
```

- [ ] **Step 2: Create `next.config.ts.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/next.config.ts.tmpl`:

```typescript
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  basePath: "/{{ .Namespace }}/{{ .Name }}",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.API_BACKEND_URL || "http://{{ .Name }}-{{ .Name }}-backend:8000"}/{{ .Namespace }}/{{ .Name }}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
```

- [ ] **Step 3: Create `package.json.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/package.json.tmpl`:

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

- [ ] **Step 4: Create `tsconfig.json.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/tsconfig.json.tmpl`:

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

- [ ] **Step 5: Create `postcss.config.mjs.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/postcss.config.mjs.tmpl`:

```javascript
const config = {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};

export default config;
```

- [ ] **Step 6: Create `globals.css.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/globals.css.tmpl`:

```css
@import "tailwindcss";
```

- [ ] **Step 7: Create `layout.tsx.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/layout.tsx.tmpl`:

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

- [ ] **Step 8: Create `page.tsx.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/src/app/page.tsx.tmpl`:

```tsx
"use client";

import { useEffect, useState, useCallback } from "react";

interface Todo {
  id: number;
  title: string;
  completed: boolean;
  created_at: string;
}

interface HealthStatus {
  status: "loading" | "ok" | "error";
  detail?: string;
}

export default function Home() {
  const [todos, setTodos] = useState<Todo[]>([]);
  const [title, setTitle] = useState("");
  const [apiHealth, setApiHealth] = useState<HealthStatus>({ status: "loading" });
  const [dbHealth, setDbHealth] = useState<HealthStatus>({ status: "loading" });

  const fetchTodos = useCallback(async () => {
    try {
      const res = await fetch("/api/todos");
      if (res.ok) setTodos(await res.json());
    } catch {}
  }, []);

  useEffect(() => {
    fetch("/api/health")
      .then((r) => r.json())
      .then((d) => setApiHealth({ status: d.status }))
      .catch(() => setApiHealth({ status: "error" }));

    fetch("/api/db-health")
      .then((r) => r.json())
      .then((d) => setDbHealth({ status: d.status, detail: d.detail }))
      .catch(() => setDbHealth({ status: "error" }));

    fetchTodos();
  }, [fetchTodos]);

  const addTodo = async () => {
    if (!title.trim()) return;
    await fetch("/api/todos", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title: title.trim() }),
    });
    setTitle("");
    fetchTodos();
  };

  const toggleTodo = async (todo: Todo) => {
    await fetch(`/api/todos/${todo.id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ completed: !todo.completed }),
    });
    fetchTodos();
  };

  const deleteTodo = async (id: number) => {
    await fetch(`/api/todos/${id}`, { method: "DELETE" });
    fetchTodos();
  };

  const dot = (s: HealthStatus) =>
    s.status === "loading"
      ? "bg-gray-500"
      : s.status === "ok"
        ? "bg-green-500"
        : "bg-red-500";

  return (
    <main className="mx-auto max-w-xl px-4 py-12">
      <h1 className="mb-6 text-3xl font-bold">{{ .Name }}</h1>

      <div className="mb-8 flex gap-6 text-sm">
        <span className="flex items-center gap-2">
          <span className={`inline-block h-2.5 w-2.5 rounded-full ${dot(apiHealth)}`} />
          API Health: {apiHealth.status}
        </span>
        <span className="flex items-center gap-2">
          <span className={`inline-block h-2.5 w-2.5 rounded-full ${dot(dbHealth)}`} />
          DB Health: {dbHealth.status}
        </span>
      </div>

      <div className="mb-6 flex gap-2">
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && addTodo()}
          placeholder="Add a todo..."
          className="flex-1 rounded bg-gray-800 px-3 py-2 text-gray-100 placeholder-gray-500 outline-none focus:ring-2 focus:ring-blue-500"
        />
        <button
          onClick={addTodo}
          className="rounded bg-blue-600 px-4 py-2 font-medium hover:bg-blue-500"
        >
          Add
        </button>
      </div>

      <ul className="space-y-2">
        {todos.map((todo) => (
          <li
            key={todo.id}
            className="flex items-center gap-3 rounded bg-gray-800/50 px-3 py-2"
          >
            <button
              onClick={() => toggleTodo(todo)}
              className={`h-5 w-5 shrink-0 rounded border ${
                todo.completed
                  ? "border-green-500 bg-green-500/20 text-green-400"
                  : "border-gray-600"
              } flex items-center justify-center text-xs`}
            >
              {todo.completed && "✓"}
            </button>
            <span
              className={`flex-1 ${todo.completed ? "text-gray-500 line-through" : ""}`}
            >
              {todo.title}
            </span>
            <span className="text-xs text-gray-600">
              {new Date(todo.created_at).toLocaleDateString()}
            </span>
            <button
              onClick={() => deleteTodo(todo.id)}
              className="text-gray-500 hover:text-red-400"
            >
              ✕
            </button>
          </li>
        ))}
      </ul>

      {todos.length === 0 && (
        <p className="mt-4 text-center text-gray-500">No todos yet.</p>
      )}
    </main>
  );
}
```

- [ ] **Step 9: Verify compilation**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 10: Commit**

```bash
git add pkg/scaffold/skeletons/nextjs-fastapi-pg/frontend/
git commit -m "feat: add nextjs-fastapi-pg frontend skeleton (Todo app with health status)"
```

---

### Task 7: Create tech-stack.yaml.tmpl and build.sh.tmpl

**Files:**
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/tech-stack.yaml.tmpl`
- Create: `pkg/scaffold/skeletons/nextjs-fastapi-pg/build.sh.tmpl`

- [ ] **Step 1: Create `tech-stack.yaml.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/tech-stack.yaml.tmpl`:

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
      - name: DATABASE_URL
        value: "postgresql+asyncpg://postgres:{{ .DBPassword }}@{{ .Name }}-postgresql:5432/{{ .Name }}"
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
    password: {{ .DBPassword }}
    imageRegistry: registry.example.com/tools
```

- [ ] **Step 2: Create `build.sh.tmpl`**

Create file `pkg/scaffold/skeletons/nextjs-fastapi-pg/build.sh.tmpl`:

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

- [ ] **Step 3: Verify compilation and run scaffold test**

Run: `go build ./... && go test ./pkg/scaffold/ -v`
Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/scaffold/skeletons/nextjs-fastapi-pg/tech-stack.yaml.tmpl pkg/scaffold/skeletons/nextjs-fastapi-pg/build.sh.tmpl
git commit -m "feat: add nextjs-fastapi-pg tech-stack.yaml and build.sh templates"
```

---

### Task 8: Add database chart template

**Files:**
- Create: `pkg/chart/templates/templates/database.yaml.tmpl`

- [ ] **Step 1: Create `database.yaml.tmpl`**

Create file `pkg/chart/templates/templates/database.yaml.tmpl`:

```
{{ "{{- if .Values.database }}" }}
{{ "{{- if .Values.database.enabled }}" }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ "{{ include \"chart.fullname\" $ }}" }}-postgresql
  labels:
    {{- "{{ include \"chart.labels\" $ | nindent 4 }}" }}
type: Opaque
stringData:
  POSTGRES_PASSWORD: {{ "{{ .Values.database.password }}" }}
  DATABASE_URL: "postgresql+asyncpg://postgres:{{ \"{{ .Values.database.password }}\" }}@{{ \"{{ include \"chart.fullname\" $ }}\" }}-postgresql:5432/{{ \"{{ .Values.database.database }}\" }}"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ "{{ include \"chart.fullname\" $ }}" }}-postgresql
  labels:
    {{- "{{ include \"chart.labels\" $ | nindent 4 }}" }}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ "{{ .Values.database.storage }}" }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ "{{ include \"chart.fullname\" $ }}" }}-postgresql
  labels:
    {{- "{{ include \"chart.labels\" $ | nindent 4 }}" }}
    app.kubernetes.io/component: postgresql
spec:
  replicas: 1
  selector:
    matchLabels:
      {{- "{{ include \"chart.selectorLabels\" $ | nindent 6 }}" }}
      app.kubernetes.io/component: postgresql
  template:
    metadata:
      labels:
        {{- "{{ include \"chart.selectorLabels\" $ | nindent 8 }}" }}
        app.kubernetes.io/component: postgresql
    spec:
      containers:
        - name: postgresql
          image: {{ "{{ .Values.database.image }}" }}
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: {{ "{{ .Values.database.database }}" }}
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ "{{ include \"chart.fullname\" $ }}" }}-postgresql
                  key: POSTGRES_PASSWORD
            - name: PGDATA
              value: /var/lib/postgresql/data/pgdata
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: {{ "{{ include \"chart.fullname\" $ }}" }}-postgresql
---
apiVersion: v1
kind: Service
metadata:
  name: {{ "{{ include \"chart.fullname\" $ }}" }}-postgresql
  labels:
    {{- "{{ include \"chart.labels\" $ | nindent 4 }}" }}
    app.kubernetes.io/component: postgresql
spec:
  ports:
    - port: 5432
      targetPort: 5432
  selector:
    {{- "{{ include \"chart.selectorLabels\" $ | nindent 4 }}" }}
    app.kubernetes.io/component: postgresql
{{ "{{- end }}" }}
{{ "{{- end }}" }}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: No errors (embedded templates are loaded automatically).

- [ ] **Step 3: Commit**

```bash
git add pkg/chart/templates/templates/database.yaml.tmpl
git commit -m "feat: add native K8s database chart template (Secret/PVC/Deployment/Service)"
```

---

### Task 9: Update chart generator to render database resources

**Files:**
- Modify: `pkg/chart/generator.go`
- Modify: `pkg/chart/templates/values.yaml.tmpl`
- Modify: `pkg/chart/templates/templates/deployment.yaml.tmpl`

- [ ] **Step 1: Add `DatabaseConfig` to `chartData` and populate it**

In `pkg/chart/generator.go`, add a new struct after `chartDependency`:

```go
type databaseConfig struct {
	Enabled  bool
	Type     string
	Image    string
	Database string
	Password string
	Storage  string
}
```

Add field to `chartData`:

```go
type chartData struct {
	Name             string
	Services         []config.Service
	Dependencies     []chartDependency
	DependencyValues map[string]string
	Database         *databaseConfig
}
```

- [ ] **Step 2: Populate `Database` in `buildChartData`**

In the `buildChartData` function, inside the `for name, dep := range ts.Dependencies` loop, before the existing `info, ok := DependencyRegistry[name]` check, add handling for native database deployment:

```go
	if name == "postgresql" && dep.ImageRegistry != "" {
		image := dep.ImageRegistry + "/postgres:" + dep.Version
		if !strings.Contains(dep.Version, "-") {
			image += "-alpine"
		}
		data.Database = &databaseConfig{
			Enabled:  true,
			Type:     "postgresql",
			Image:    image,
			Database: dep.Database,
			Password: dep.Password,
			Storage:  dep.Storage,
		}
		if data.Database.Storage == "" {
			data.Database.Storage = "1Gi"
		}
		if data.Database.Database == "" {
			data.Database.Database = ts.ProjectName()
		}
		continue
	}
```

Add `"strings"` to the imports if not already present.

- [ ] **Step 3: Add `database.yaml.tmpl` to render list**

In the `Generate` function, add to the `files` map:

```go
"templates/templates/database.yaml.tmpl": filepath.Join(outDir, "templates", "database.yaml"),
```

- [ ] **Step 4: Add database section to `values.yaml.tmpl`**

In `pkg/chart/templates/values.yaml.tmpl`, add before the dependency values range block (before `{{ range $key, $vals := .DependencyValues }}`):

```
{{- if .Database }}
database:
  enabled: {{ .Database.Enabled }}
  type: {{ .Database.Type }}
  image: {{ .Database.Image }}
  database: {{ .Database.Database }}
  password: {{ .Database.Password }}
  storage: {{ .Database.Storage }}
{{- end }}
```

- [ ] **Step 5: Add `envFrom` support to deployment template**

In `pkg/chart/templates/templates/deployment.yaml.tmpl`, after the `env` block (after `{{ "{{- end }}" }}` that closes the `if $svc.env` block), add:

```
          {{ "{{- if $svc.envFrom }}" }}
          envFrom:
            {{- "{{ toYaml $svc.envFrom | nindent 12 }}" }}
          {{ "{{- end }}" }}
```

- [ ] **Step 6: Run tests**

Run: `go test ./pkg/chart/ -v`
Expected: All existing tests PASS. The new database field is nil for existing tests so no impact.

- [ ] **Step 7: Commit**

```bash
git add pkg/chart/generator.go pkg/chart/templates/values.yaml.tmpl pkg/chart/templates/templates/deployment.yaml.tmpl
git commit -m "feat: chart generator renders native database resources for postgresql"
```

---

### Task 10: Add chart generator test for PostgreSQL dependency

**Files:**
- Modify: `pkg/chart/generator_test.go`

- [ ] **Step 1: Add test for native postgresql deployment**

In `pkg/chart/generator_test.go`, add:

```go
func TestGenerate_WithNativePostgresql(t *testing.T) {
	outDir := t.TempDir()

	ts := &config.TechStack{
		Name: "myapp",
		Services: []config.Service{
			{
				Name:     "myapp-backend",
				Image:    "registry/myapp-backend:latest",
				Port:     8000,
				Replicas: 1,
				Env: []config.EnvVar{
					{Name: "DATABASE_URL", Value: "postgresql+asyncpg://postgres:secret@myapp-postgresql:5432/myapp"},
				},
				Resources: config.Resources{CPU: "250m", Memory: "256Mi"},
			},
		},
		Dependencies: map[string]*config.Dependency{
			"postgresql": {
				Version:       "16",
				Database:      "myapp",
				Password:      "secret",
				Storage:       "2Gi",
				ImageRegistry: "harbor.example.com/tools",
			},
		},
	}

	if err := Generate(ts, outDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Chart.yaml should NOT have bitnami dependency
	chartYaml, _ := os.ReadFile(filepath.Join(outDir, "Chart.yaml"))
	if strings.Contains(string(chartYaml), "bitnami") {
		t.Error("Chart.yaml should not have bitnami dependency for native postgresql")
	}

	// values.yaml should have database section
	valuesYaml, _ := os.ReadFile(filepath.Join(outDir, "values.yaml"))
	valuesContent := string(valuesYaml)
	if !strings.Contains(valuesContent, "database:") {
		t.Error("values.yaml missing database section")
	}
	if !strings.Contains(valuesContent, "harbor.example.com/tools/postgres:16-alpine") {
		t.Error("values.yaml missing correct database image")
	}
	if !strings.Contains(valuesContent, "storage: 2Gi") {
		t.Error("values.yaml missing storage")
	}

	// database.yaml should exist with all resources
	dbYaml, err := os.ReadFile(filepath.Join(outDir, "templates", "database.yaml"))
	if err != nil {
		t.Fatalf("database.yaml missing: %v", err)
	}
	dbContent := string(dbYaml)
	if !strings.Contains(dbContent, "kind: Secret") {
		t.Error("database.yaml missing Secret")
	}
	if !strings.Contains(dbContent, "kind: PersistentVolumeClaim") {
		t.Error("database.yaml missing PVC")
	}
	if !strings.Contains(dbContent, "kind: Deployment") {
		t.Error("database.yaml missing Deployment")
	}
	if !strings.Contains(dbContent, "kind: Service") {
		t.Error("database.yaml missing Service")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./pkg/chart/ -v`
Expected: All PASS.

- [ ] **Step 3: Commit**

```bash
git add pkg/chart/generator_test.go
git commit -m "test: add chart generator test for native postgresql deployment"
```

---

### Task 11: End-to-end scaffold test

**Files:**
- Modify: `pkg/scaffold/scaffold_test.go`

- [ ] **Step 1: Read current scaffold test to understand patterns**

Read `pkg/scaffold/scaffold_test.go` to see existing test structure.

- [ ] **Step 2: Add scaffold test for nextjs-fastapi-pg**

Add to `pkg/scaffold/scaffold_test.go`:

```go
func TestRenderSkeleton_NextjsFastapiPg(t *testing.T) {
	outDir := t.TempDir()
	params := Params{
		Name:         "testapp",
		Namespace:    "sandbox",
		Registry:     "harbor.example.com/ns",
		Domain:       "example.com",
		BaseRegistry: "harbor.example.com/baselibrary",
		DBPassword:   "testpass123",
	}

	if err := RenderSkeleton("nextjs-fastapi-pg", params, outDir); err != nil {
		t.Fatalf("RenderSkeleton failed: %v", err)
	}

	// Check backend files exist and have correct content
	mainPy, err := os.ReadFile(filepath.Join(outDir, "backend", "main.py"))
	if err != nil {
		t.Fatalf("backend/main.py missing: %v", err)
	}
	mainContent := string(mainPy)
	if !strings.Contains(mainContent, "testapp-postgresql:5432/testapp") {
		t.Error("main.py missing correct DATABASE_URL")
	}
	if !strings.Contains(mainContent, "testpass123") {
		t.Error("main.py missing DB password")
	}
	if !strings.Contains(mainContent, "/sandbox/testapp/api") {
		t.Error("main.py missing correct API prefix")
	}

	// Check frontend files exist
	pageTsx, err := os.ReadFile(filepath.Join(outDir, "frontend", "src", "app", "page.tsx"))
	if err != nil {
		t.Fatalf("page.tsx missing: %v", err)
	}
	if !strings.Contains(string(pageTsx), "/api/db-health") {
		t.Error("page.tsx missing db-health check")
	}
	if !strings.Contains(string(pageTsx), "/api/todos") {
		t.Error("page.tsx missing todos API calls")
	}

	// Check tech-stack.yaml
	techStack, err := os.ReadFile(filepath.Join(outDir, "tech-stack.yaml"))
	if err != nil {
		t.Fatalf("tech-stack.yaml missing: %v", err)
	}
	tsContent := string(techStack)
	if !strings.Contains(tsContent, "postgresql:") {
		t.Error("tech-stack.yaml missing postgresql dependency")
	}
	if !strings.Contains(tsContent, "testpass123") {
		t.Error("tech-stack.yaml missing DB password")
	}
	if !strings.Contains(tsContent, "imageRegistry:") {
		t.Error("tech-stack.yaml missing imageRegistry")
	}

	// Check build.sh is executable
	buildSh := filepath.Join(outDir, "build.sh")
	info, err := os.Stat(buildSh)
	if err != nil {
		t.Fatalf("build.sh missing: %v", err)
	}
	if info.Mode()&0100 == 0 {
		t.Error("build.sh is not executable")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/scaffold/ -v`
Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/scaffold/scaffold_test.go
git commit -m "test: add end-to-end scaffold test for nextjs-fastapi-pg template"
```

---

### Task 12: Run full test suite and build binary

**Files:** None (verification only)

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS.

- [ ] **Step 2: Build binary**

Run: `go build -o vela main.go`
Expected: Binary builds successfully.

- [ ] **Step 3: Smoke test**

Run: `./vela create testpg -t nextjs-fastapi-pg --namespace sandbox`
Expected: Creates `testpg/` directory with all files. Check `testpg/.vela/state.yaml` has credentials section.

Run: `cd testpg && ../vela credentials`
Expected: Displays PostgreSQL credentials with host, port, database, user, password, URL.

Clean up: `rm -rf testpg`

- [ ] **Step 4: Commit any fixes, then final commit**

```bash
git add -A
git commit -m "feat: nextjs-fastapi-pg template complete — Todo CRUD app with native PostgreSQL"
```
