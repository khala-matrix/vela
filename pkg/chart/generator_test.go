package chart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mars/vela/pkg/config"
)

func TestGenerate_BasicApp(t *testing.T) {
	outDir := t.TempDir()

	ts := &config.TechStack{
		Name: "myapp",
		Services: []config.Service{
			{
				Name:     "myapp",
				Image:    "myapp:v1.0.0",
				Port:     8080,
				Replicas: 1,
				Resources: config.Resources{
					CPU:    "250m",
					Memory: "256Mi",
				},
			},
		},
	}

	if err := Generate(ts, outDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	chartYaml, err := os.ReadFile(filepath.Join(outDir, "Chart.yaml"))
	if err != nil {
		t.Fatalf("Chart.yaml missing: %v", err)
	}
	if !strings.Contains(string(chartYaml), "name: myapp") {
		t.Error("Chart.yaml missing app name")
	}

	valuesYaml, err := os.ReadFile(filepath.Join(outDir, "values.yaml"))
	if err != nil {
		t.Fatalf("values.yaml missing: %v", err)
	}
	content := string(valuesYaml)
	if !strings.Contains(content, "image: myapp:v1.0.0") {
		t.Error("values.yaml missing image")
	}
	if !strings.Contains(content, "port: 8080") {
		t.Error("values.yaml missing port")
	}
	if !strings.Contains(content, "services:") {
		t.Error("values.yaml missing services key")
	}

	deployYaml, err := os.ReadFile(filepath.Join(outDir, "templates", "deployment.yaml"))
	if err != nil {
		t.Fatalf("deployment.yaml missing: %v", err)
	}
	if !strings.Contains(string(deployYaml), "kind: Deployment") {
		t.Error("deployment.yaml invalid")
	}

	svcYaml, err := os.ReadFile(filepath.Join(outDir, "templates", "service.yaml"))
	if err != nil {
		t.Fatalf("service.yaml missing: %v", err)
	}
	if !strings.Contains(string(svcYaml), "kind: Service") {
		t.Error("service.yaml invalid")
	}

	_, err = os.ReadFile(filepath.Join(outDir, "templates", "_helpers.tpl"))
	if err != nil {
		t.Fatalf("_helpers.tpl missing: %v", err)
	}
}

func TestGenerate_WithDependencies(t *testing.T) {
	outDir := t.TempDir()

	ts := &config.TechStack{
		Name: "myapp",
		Services: []config.Service{
			{
				Name:     "myapp",
				Image:    "myapp:v1.0.0",
				Port:     8080,
				Replicas: 1,
				Resources: config.Resources{
					CPU:    "250m",
					Memory: "256Mi",
				},
			},
		},
		Dependencies: map[string]*config.Dependency{
			"mysql": {
				Version:  "8.0",
				Storage:  "5Gi",
				Password: "secret",
				Database: "mydb",
			},
		},
	}

	if err := Generate(ts, outDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	chartYaml, _ := os.ReadFile(filepath.Join(outDir, "Chart.yaml"))
	content := string(chartYaml)
	if !strings.Contains(content, "name: mysql") {
		t.Error("Chart.yaml missing mysql dependency")
	}
	if !strings.Contains(content, "charts.bitnami.com") {
		t.Error("Chart.yaml missing bitnami repository")
	}
}

func TestGenerate_WithEnvVars(t *testing.T) {
	outDir := t.TempDir()

	ts := &config.TechStack{
		Name: "myapp",
		Services: []config.Service{
			{
				Name:     "myapp",
				Image:    "myapp:v1.0.0",
				Port:     8080,
				Replicas: 1,
				Env: []config.EnvVar{
					{Name: "APP_ENV", Value: "development"},
				},
				Resources: config.Resources{
					CPU:    "250m",
					Memory: "256Mi",
				},
			},
		},
	}

	if err := Generate(ts, outDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	valuesYaml, _ := os.ReadFile(filepath.Join(outDir, "values.yaml"))
	content := string(valuesYaml)
	if !strings.Contains(content, "APP_ENV") {
		t.Error("values.yaml missing env var")
	}
}

func TestGenerate_MultiService(t *testing.T) {
	outDir := t.TempDir()

	ts := &config.TechStack{
		Name: "fullstack-app",
		Services: []config.Service{
			{
				Name:     "fastapi-backend",
				Image:    "registry.example.com/fastapi-backend:latest",
				Port:     8000,
				Replicas: 2,
				Env: []config.EnvVar{
					{Name: "CORS_ORIGINS", Value: "http://localhost:3000"},
				},
				Resources: config.Resources{
					CPU:    "500m",
					Memory: "512Mi",
				},
			},
			{
				Name:     "nextjs-frontend",
				Image:    "registry.example.com/nextjs-frontend:latest",
				Port:     3000,
				Replicas: 1,
				Resources: config.Resources{
					CPU:    "250m",
					Memory: "256Mi",
				},
			},
		},
	}

	if err := Generate(ts, outDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	chartYaml, _ := os.ReadFile(filepath.Join(outDir, "Chart.yaml"))
	if !strings.Contains(string(chartYaml), "name: fullstack-app") {
		t.Error("Chart.yaml missing project name")
	}

	valuesYaml, _ := os.ReadFile(filepath.Join(outDir, "values.yaml"))
	content := string(valuesYaml)
	if !strings.Contains(content, "fastapi-backend:") {
		t.Error("values.yaml missing fastapi-backend service")
	}
	if !strings.Contains(content, "nextjs-frontend:") {
		t.Error("values.yaml missing nextjs-frontend service")
	}
	if !strings.Contains(content, "image: registry.example.com/fastapi-backend:latest") {
		t.Error("values.yaml missing backend image")
	}
	if !strings.Contains(content, "port: 3000") {
		t.Error("values.yaml missing frontend port")
	}
	if !strings.Contains(content, "CORS_ORIGINS") {
		t.Error("values.yaml missing backend env var")
	}

	deployYaml, _ := os.ReadFile(filepath.Join(outDir, "templates", "deployment.yaml"))
	deployContent := string(deployYaml)
	if !strings.Contains(deployContent, "app.kubernetes.io/component") {
		t.Error("deployment.yaml missing component label")
	}
	if !strings.Contains(deployContent, "range $name, $svc := .Values.services") {
		t.Error("deployment.yaml missing services range loop")
	}

	svcYaml, _ := os.ReadFile(filepath.Join(outDir, "templates", "service.yaml"))
	svcContent := string(svcYaml)
	if !strings.Contains(svcContent, "range $name, $svc := .Values.services") {
		t.Error("service.yaml missing services range loop")
	}
}
