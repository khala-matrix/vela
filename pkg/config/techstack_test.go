package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTechStack_Full(t *testing.T) {
	yaml := `
app:
  name: myapp
  image: registry.example.com/myapp:v1.0.0
  port: 8080
  replicas: 2
  env:
    - name: APP_ENV
      value: development
  resources:
    cpu: 500m
    memory: 512Mi
dependencies:
  mysql:
    version: "8.0"
    storage: 5Gi
    password: secret
    database: mydb
  redis:
    version: "7.0"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	ts, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if ts.App.Name != "myapp" {
		t.Errorf("expected name myapp, got %s", ts.App.Name)
	}
	if ts.App.Image != "registry.example.com/myapp:v1.0.0" {
		t.Errorf("unexpected image: %s", ts.App.Image)
	}
	if ts.App.Port != 8080 {
		t.Errorf("expected port 8080, got %d", ts.App.Port)
	}
	if ts.App.Replicas != 2 {
		t.Errorf("expected replicas 2, got %d", ts.App.Replicas)
	}
	if len(ts.App.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(ts.App.Env))
	}
	if ts.App.Env[0].Name != "APP_ENV" {
		t.Errorf("unexpected env name: %s", ts.App.Env[0].Name)
	}
	if ts.App.Resources.CPU != "500m" {
		t.Errorf("unexpected cpu: %s", ts.App.Resources.CPU)
	}

	mysql, ok := ts.Dependencies["mysql"]
	if !ok {
		t.Fatal("mysql dependency missing")
	}
	if mysql.Version != "8.0" {
		t.Errorf("unexpected mysql version: %s", mysql.Version)
	}
	if mysql.Storage != "5Gi" {
		t.Errorf("unexpected mysql storage: %s", mysql.Storage)
	}
	if mysql.Password != "secret" {
		t.Errorf("unexpected mysql password: %s", mysql.Password)
	}
	if mysql.Database != "mydb" {
		t.Errorf("unexpected mysql database: %s", mysql.Database)
	}

	redis, ok := ts.Dependencies["redis"]
	if !ok {
		t.Fatal("redis dependency missing")
	}
	if redis.Version != "7.0" {
		t.Errorf("unexpected redis version: %s", redis.Version)
	}
}

func TestParseTechStack_Defaults(t *testing.T) {
	yaml := `
app:
  name: minimal
  image: myimg:latest
  port: 3000
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	ts, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if ts.App.Replicas != 1 {
		t.Errorf("expected default replicas 1, got %d", ts.App.Replicas)
	}
	if ts.App.Resources.CPU != "250m" {
		t.Errorf("expected default cpu 250m, got %s", ts.App.Resources.CPU)
	}
	if ts.App.Resources.Memory != "256Mi" {
		t.Errorf("expected default memory 256Mi, got %s", ts.App.Resources.Memory)
	}
}

func TestParseTechStack_MissingRequired(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"missing name", `
app:
  image: myimg:latest
  port: 3000
`},
		{"missing image", `
app:
  name: myapp
  port: 3000
`},
		{"missing port", `
app:
  name: myapp
  image: myimg:latest
`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "tech-stack.yaml")
			os.WriteFile(path, []byte(tc.yaml), 0644)

			_, err := Parse(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseTechStack_UnsupportedDependency(t *testing.T) {
	yaml := `
app:
  name: myapp
  image: myimg:latest
  port: 3000
dependencies:
  rabbitmq:
    version: "3.12"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for unsupported dependency")
	}
}

func TestParseTechStack_FileNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseTechStack_MultiService(t *testing.T) {
	yaml := `
name: fullstack-app
services:
  - name: fastapi-backend
    image: registry.example.com/fastapi-backend:latest
    port: 8000
    replicas: 2
    env:
      - name: CORS_ORIGINS
        value: "http://localhost:3000"
    resources:
      cpu: 500m
      memory: 512Mi
  - name: nextjs-frontend
    image: registry.example.com/nextjs-frontend:latest
    port: 3000
    env:
      - name: NEXT_PUBLIC_API_URL
        value: "http://fastapi-backend:8000"
dependencies:
  redis:
    version: "7.0"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	ts, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if ts.ProjectName() != "fullstack-app" {
		t.Errorf("expected project name fullstack-app, got %s", ts.ProjectName())
	}
	if !ts.IsMultiService() {
		t.Error("expected IsMultiService() to be true")
	}
	if len(ts.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(ts.Services))
	}

	backend := ts.Services[0]
	if backend.Name != "fastapi-backend" {
		t.Errorf("expected backend name fastapi-backend, got %s", backend.Name)
	}
	if backend.Port != 8000 {
		t.Errorf("expected backend port 8000, got %d", backend.Port)
	}
	if backend.Replicas != 2 {
		t.Errorf("expected backend replicas 2, got %d", backend.Replicas)
	}

	frontend := ts.Services[1]
	if frontend.Name != "nextjs-frontend" {
		t.Errorf("expected frontend name nextjs-frontend, got %s", frontend.Name)
	}
	if frontend.Replicas != 1 {
		t.Errorf("expected default replicas 1, got %d", frontend.Replicas)
	}
	if frontend.Resources.CPU != "250m" {
		t.Errorf("expected default cpu 250m, got %s", frontend.Resources.CPU)
	}

	redis, ok := ts.Dependencies["redis"]
	if !ok {
		t.Fatal("redis dependency missing")
	}
	if redis.Version != "7.0" {
		t.Errorf("unexpected redis version: %s", redis.Version)
	}
}

func TestParseTechStack_LegacyConvertsToServices(t *testing.T) {
	yaml := `
app:
  name: myapp
  image: myimg:latest
  port: 3000
  replicas: 2
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	ts, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if ts.ProjectName() != "myapp" {
		t.Errorf("expected project name myapp, got %s", ts.ProjectName())
	}
	if len(ts.Services) != 1 {
		t.Fatalf("expected 1 service from legacy conversion, got %d", len(ts.Services))
	}

	svc := ts.Services[0]
	if svc.Name != "myapp" {
		t.Errorf("expected service name myapp, got %s", svc.Name)
	}
	if svc.Image != "myimg:latest" {
		t.Errorf("expected image myimg:latest, got %s", svc.Image)
	}
	if svc.Port != 3000 {
		t.Errorf("expected port 3000, got %d", svc.Port)
	}
	if svc.Replicas != 2 {
		t.Errorf("expected replicas 2, got %d", svc.Replicas)
	}
}

func TestParseTechStack_MultiService_DuplicateName(t *testing.T) {
	yaml := `
name: myproject
services:
  - name: api
    image: api:latest
    port: 8000
  - name: api
    image: api-v2:latest
    port: 8001
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for duplicate service name")
	}
}

func TestParseTechStack_MultiService_MissingName(t *testing.T) {
	yaml := `
services:
  - name: api
    image: api:latest
    port: 8000
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for missing top-level name")
	}
}

func TestParseTechStack_DependencyDefaults(t *testing.T) {
	yaml := `
app:
  name: myapp
  image: myimg:latest
  port: 3000
dependencies:
  mysql:
    version: "8.0"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tech-stack.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	ts, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	mysql := ts.Dependencies["mysql"]
	if mysql.Storage != "1Gi" {
		t.Errorf("expected default storage 1Gi, got %s", mysql.Storage)
	}
	if mysql.Database != "myapp" {
		t.Errorf("expected default database myapp, got %s", mysql.Database)
	}
}

func TestParseBytes_Valid(t *testing.T) {
	yaml := []byte(`
name: test-app
ingress:
  host: apps.example.com
services:
  - name: test-app-backend
    image: registry.example.com/test-app-backend:latest
    port: 8000
    ingress:
      enabled: true
      path: /sandbox/test-app/api
      stripPrefix: false
  - name: test-app-frontend
    image: registry.example.com/test-app-frontend:latest
    port: 3000
    ingress:
      enabled: true
      path: /sandbox/test-app
      stripPrefix: false
`)
	ts, err := ParseBytes(yaml)
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}
	if ts.ProjectName() != "test-app" {
		t.Errorf("expected project name test-app, got %s", ts.ProjectName())
	}
	if len(ts.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(ts.Services))
	}
}

func TestParseBytes_Invalid(t *testing.T) {
	yaml := []byte(`
name: test-app
services:
  - name: broken
    port: 8000
`)
	_, err := ParseBytes(yaml)
	if err == nil {
		t.Fatal("expected validation error for missing image")
	}
}
