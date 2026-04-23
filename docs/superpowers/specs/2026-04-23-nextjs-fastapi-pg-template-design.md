# Next.js + FastAPI + PostgreSQL Template Design

## Goal

Add a new scaffold template `nextjs-fastapi-pg` that generates a full-stack project with a Todo CRUD app backed by PostgreSQL. Database is deployed as a native K8s resource (not Bitnami subchart), with images sourced from the user's base registry. Passwords are auto-generated and accessible via a new `vela credentials` command.

## Architecture

```
Frontend (Next.js :3000)  →  Backend (FastAPI :8000)  →  PostgreSQL (:5432)
          basePath: /ns/app       prefix: /ns/app/api       PVC + Secret
```

All three components are deployed to the same namespace. Frontend proxies API calls via Next.js rewrites. Backend connects to PostgreSQL via `DATABASE_URL` env var injected from a K8s Secret. PostgreSQL credentials are auto-generated at `vela create` time.

## Components

### 1. Scaffold Template: `pkg/scaffold/skeletons/nextjs-fastapi-pg/`

Independent directory, no sharing with `nextjs-fastapi`. File tree:

```
nextjs-fastapi-pg/
  tech-stack.yaml.tmpl
  build.sh.tmpl
  backend/
    Dockerfile.tmpl
    main.py.tmpl
    requirements.txt.tmpl
  frontend/
    Dockerfile.tmpl
    next.config.ts.tmpl
    package.json.tmpl
    tsconfig.json.tmpl
    postcss.config.mjs.tmpl
    src/app/
      globals.css.tmpl
      layout.tsx.tmpl
      page.tsx.tmpl
```

Template registered in `pkg/scaffold/scaffold.go`:

```go
{ID: "nextjs-fastapi-pg", Name: "Next.js + FastAPI + PostgreSQL", Description: "Full-stack web app with database — Python backend, React frontend, PostgreSQL"}
```

### 2. Backend: FastAPI + SQLAlchemy Async

**`requirements.txt.tmpl`:**

```
fastapi==0.115.12
uvicorn==0.34.2
sqlalchemy[asyncio]==2.0.40
asyncpg==0.30.0
```

**`main.py.tmpl`:**

- SQLAlchemy async engine + async_sessionmaker, connecting to `DATABASE_URL` env var
- Default `DATABASE_URL`: `postgresql+asyncpg://postgres:<password>@{{ .Name }}-postgresql:5432/{{ .Name }}`
- `Todo` model: `id` (int, PK), `title` (str), `completed` (bool, default false), `created_at` (datetime)
- Startup event: `create_all` to auto-create tables
- CORS middleware (same as existing template)
- Routes under `APIRouter(prefix="/{{ .Namespace }}/{{ .Name }}/api")`:
  - `GET /health` — returns `{"status": "ok"}`
  - `GET /db-health` — executes `SELECT 1`, returns `{"status": "ok"}` or `{"status": "error", "detail": "..."}`
  - `GET /todos` — list all todos, ordered by created_at desc
  - `POST /todos` — create todo, body: `{"title": "..."}`, returns created todo
  - `PATCH /todos/{id}` — update todo, body: `{"title": "...", "completed": true/false}`, returns updated todo
  - `DELETE /todos/{id}` — delete todo, returns `{"ok": true}`

**`Dockerfile.tmpl`:** Same pattern as existing `nextjs-fastapi` — base image from `BaseRegistry/python:3.12-slim-bookworm`, `USER 1000` at runtime.

### 3. Frontend: Next.js Todo App

**`next.config.ts.tmpl`:** Same as existing — `basePath: "/{{ .Namespace }}/{{ .Name }}"`, rewrites `/api/:path*` to backend.

**`page.tsx.tmpl`:** Single-page Todo app with:

- **Health status bar** at top: two indicators showing API Health and DB Health status
  - On page load, fetches `/api/health` and `/api/db-health`
  - Displays green "ok" or red "error" for each
- **Todo list**: displays all todos with title, completed status, created_at
- **Add form**: text input + submit button to create new todo
- **Toggle**: click to toggle completed status (PATCH)
- **Delete**: button per todo to delete (DELETE)
- All API calls go through `/api/...` (proxied by Next.js rewrites to backend)

**`Dockerfile.tmpl`:** Same pattern as existing — multi-stage build, `USER root` for build, `USER 1000` for runtime.

Other frontend files (`package.json.tmpl`, `tsconfig.json.tmpl`, `postcss.config.mjs.tmpl`, `globals.css.tmpl`, `layout.tsx.tmpl`) are copied from `nextjs-fastapi` with `{{ .Name }}` references preserved.

### 4. tech-stack.yaml.tmpl

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
```

### 5. Scaffold Params

Add `DBPassword` field to `pkg/scaffold/scaffold.go`:

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

### 6. PostgreSQL Chart Resources (Native K8s, Not Bitnami)

When `dependencies` contains `postgresql`, the chart generator renders a new template `database.yaml.tmpl` that produces:

**Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <release>-postgresql
type: Opaque
stringData:
  POSTGRES_PASSWORD: <from values>
  DATABASE_URL: postgresql+asyncpg://postgres:<pw>@<release>-postgresql:5432/<db>
```

**PVC:**
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: <release>-postgresql
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: <from values, default 1Gi>
```

**Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: <release>-postgresql
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: postgresql
          image: <imageRegistry>/postgres:<version>
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: <database>
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: <release>-postgresql
                  key: POSTGRES_PASSWORD
            - name: PGDATA
              value: /var/lib/postgresql/data/pgdata
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: <release>-postgresql
```

**Service:**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: <release>-postgresql
spec:
  ports:
    - port: 5432
  selector: ...
```

### 7. Chart Generator Changes

**`pkg/chart/generator.go`:**

- Add `database.yaml.tmpl` to the rendered files list
- When `dependencies` contains a DB type (postgresql/mysql/mongodb), render `database.yaml.tmpl`
- Pass DB config (image registry, version, database name, password, storage) into chart data

**`pkg/chart/templates/values.yaml.tmpl`:**

Add database section when dependency exists:

```yaml
database:
  enabled: true
  type: postgresql
  image: <baseRegistry>/postgres:16-alpine
  database: <name>
  storage: 1Gi
```

**Backend service env** in values.yaml references the Secret:

```yaml
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: <release>-postgresql
      key: DATABASE_URL
- name: PG_PASSWORD
  valueFrom:
    secretKeyRef:
      name: <release>-postgresql
      key: POSTGRES_PASSWORD
```

The scaffold `tech-stack.yaml` keeps the plain `DATABASE_URL` env var for readability. At deploy time, the chart generator detects the postgresql dependency and adds `envFrom` referencing the K8s Secret, which injects `DATABASE_URL` and `PG_PASSWORD` — the Secret values take precedence over plain env vars.

### 8. Password Lifecycle

1. **`vela create`** — generates random 16-char alphanumeric password, passes as `Params.DBPassword`
   - Written into `tech-stack.yaml` (under `dependencies.postgresql.password`)
   - Written into `.vela/state.yaml` (new `credentials` section)
2. **`vela deploy`** — `config.Parse()` reads password from `tech-stack.yaml`, chart generator creates K8s Secret
3. **`vela credentials`** — reads `.vela/state.yaml` and displays credentials

### 9. State Changes

**`.vela/state.yaml`** new `credentials` field:

```yaml
name: my-app
namespace: sandbox
status: deployed
credentials:
  postgresql:
    host: my-app-postgresql
    port: 5432
    database: my-app
    user: postgres
    password: aB3x9kF2mQ...
```

`pkg/state/state.go` — add `Credentials` map to `State` struct.

### 10. New Command: `vela credentials`

**`cmd/credentials.go`:**

```
$ vela credentials

PostgreSQL:
  Host:     my-app-postgresql:5432
  Database: my-app
  User:     postgres
  Password: aB3x9kF2mQ7pL1nR8wD
  URL:      postgresql://postgres:aB3x9kF2mQ7pL1nR8wD@my-app-postgresql:5432/my-app
```

Reads from `.vela/state.yaml`. If no credentials found, prints "No credentials configured."

### 11. Deployment Template Changes

**`pkg/chart/templates/templates/deployment.yaml.tmpl`:**

Add support for `envFrom` to inject all keys from a Secret as env vars. After the existing `env` block, add:

```yaml
{{- if $svc.envFrom }}
envFrom:
{{- toYaml $svc.envFrom | nindent 12 }}
{{- end }}
```

**`pkg/config/techstack.go`** — add `EnvFrom` to `Service`:

```go
type EnvFrom struct {
    SecretRef string `yaml:"secretRef"`
}
```

The chart generator detects services with a `DATABASE_URL` env var pointing to a postgresql dependency, and adds an `envFrom` entry referencing the postgresql Secret. The backend gets `DATABASE_URL` and `PG_PASSWORD` injected from the Secret, overriding any plain `value` from the env list.

The scaffold `tech-stack.yaml` keeps the plain `DATABASE_URL` env var for readability (users can see what the connection string looks like). At deploy time, the Secret values take precedence.

### 12. Base Registry for Database Image

The database image uses `BaseRegistry` from `tech-stack.yaml` or from the chart generation context. Since `BaseRegistry` is a scaffold-time parameter (not in `tech-stack.yaml`), we need to pass it to the chart generator.

**Approach:** Add an `imageRegistry` field to the dependency config in `tech-stack.yaml`:

```yaml
dependencies:
  postgresql:
    version: "16"
    database: my-app
    password: ...
    storage: 1Gi
    imageRegistry: harbor.cn.svc.corpintra.net/tools
```

The scaffold template writes `BaseRegistry` into this field. The chart generator reads it and uses it for the PostgreSQL container image.

Add `ImageRegistry` field to `pkg/config/techstack.go` `Dependency` struct:

```go
type Dependency struct {
    Version       string `yaml:"version"`
    Storage       string `yaml:"storage,omitempty"`
    Password      string `yaml:"password,omitempty"`
    Database      string `yaml:"database,omitempty"`
    ImageRegistry string `yaml:"imageRegistry,omitempty"`
}
```

## Out of Scope

- MySQL/MongoDB/Redis native deployment (only postgresql for now; others keep Bitnami subchart path)
- Database migrations tooling
- Backup/restore
- Connection pooling (PgBouncer)
- Multiple database instances
