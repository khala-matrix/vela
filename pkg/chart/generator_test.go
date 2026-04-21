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
		App: config.App{
			Name:     "myapp",
			Image:    "myapp:v1.0.0",
			Port:     8080,
			Replicas: 1,
			Resources: config.Resources{
				CPU:    "250m",
				Memory: "256Mi",
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
	if !strings.Contains(string(valuesYaml), "image: myapp:v1.0.0") {
		t.Error("values.yaml missing image")
	}
	if !strings.Contains(string(valuesYaml), "port: 8080") {
		t.Error("values.yaml missing port")
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
		App: config.App{
			Name:     "myapp",
			Image:    "myapp:v1.0.0",
			Port:     8080,
			Replicas: 1,
			Resources: config.Resources{
				CPU:    "250m",
				Memory: "256Mi",
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
		App: config.App{
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
