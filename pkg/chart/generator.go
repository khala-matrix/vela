package chart

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mars/vela/pkg/config"
	"gopkg.in/yaml.v3"
)

//go:embed all:templates
var templateFS embed.FS

type chartData struct {
	Name             string
	Services         []config.Service
	Dependencies     []chartDependency
	DependencyValues map[string]string
	Database         *databaseConfig
}

type chartDependency struct {
	ChartName  string
	Version    string
	Repository string
}

type databaseConfig struct {
	Enabled  bool
	Type     string
	Image    string
	Database string
	Password string
	Storage  string
}

func Generate(ts *config.TechStack, outDir string) error {
	data := buildChartData(ts)

	if err := os.MkdirAll(filepath.Join(outDir, "templates"), 0755); err != nil {
		return fmt.Errorf("create chart directories: %w", err)
	}

	files := map[string]string{
		"templates/Chart.yaml.tmpl":                filepath.Join(outDir, "Chart.yaml"),
		"templates/values.yaml.tmpl":               filepath.Join(outDir, "values.yaml"),
		"templates/templates/deployment.yaml.tmpl": filepath.Join(outDir, "templates", "deployment.yaml"),
		"templates/templates/service.yaml.tmpl":    filepath.Join(outDir, "templates", "service.yaml"),
		"templates/templates/_helpers.tpl.tmpl":    filepath.Join(outDir, "templates", "_helpers.tpl"),
		"templates/templates/ingress.yaml.tmpl":    filepath.Join(outDir, "templates", "ingress.yaml"),
		"templates/templates/database.yaml.tmpl":   filepath.Join(outDir, "templates", "database.yaml"),
	}

	for tmplPath, outPath := range files {
		if err := renderTemplate(tmplPath, outPath, data); err != nil {
			return fmt.Errorf("render %s: %w", tmplPath, err)
		}
	}

	return nil
}

func buildChartData(ts *config.TechStack) chartData {
	data := chartData{
		Name:             ts.ProjectName(),
		Services:         ts.Services,
		DependencyValues: make(map[string]string),
	}

	for name, dep := range ts.Dependencies {
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

		info, ok := DependencyRegistry[name]
		if !ok {
			continue
		}

		data.Dependencies = append(data.Dependencies, chartDependency{
			ChartName:  info.ChartName,
			Version:    resolveChartVersion(name, dep.Version),
			Repository: info.Repository,
		})

		values := buildDependencyValues(name, dep, info)
		yamlBytes, _ := yaml.Marshal(values)
		data.DependencyValues[name] = string(yamlBytes)
	}

	return data
}

func resolveChartVersion(name string, _ string) string {
	versions := map[string]string{
		"mysql":      "11.1.17",
		"postgresql": "15.5.38",
		"redis":      "19.6.4",
		"mongodb":    "15.6.24",
	}
	if v, ok := versions[name]; ok {
		return v
	}
	return "latest"
}

func buildDependencyValues(name string, dep *config.Dependency, info DependencyInfo) map[string]any {
	switch name {
	case "mysql":
		vals := map[string]any{
			"auth": map[string]any{
				"database": dep.Database,
			},
			"primary": map[string]any{
				"persistence": map[string]any{
					"size": dep.Storage,
				},
			},
		}
		if dep.Password != "" {
			vals["auth"].(map[string]any)["rootPassword"] = dep.Password
		}
		return vals
	case "postgresql":
		vals := map[string]any{
			"auth": map[string]any{
				"database": dep.Database,
			},
			"primary": map[string]any{
				"persistence": map[string]any{
					"size": dep.Storage,
				},
			},
		}
		if dep.Password != "" {
			vals["auth"].(map[string]any)["postgresPassword"] = dep.Password
		}
		return vals
	case "redis":
		vals := map[string]any{
			"architecture": "standalone",
			"master": map[string]any{
				"persistence": map[string]any{
					"size": dep.Storage,
				},
			},
		}
		if dep.Password != "" {
			vals["auth"] = map[string]any{
				"password": dep.Password,
			}
		}
		return vals
	case "mongodb":
		vals := map[string]any{
			"architecture": "standalone",
			"persistence": map[string]any{
				"size": dep.Storage,
			},
		}
		if dep.Database != "" {
			vals["auth"] = map[string]any{
				"databases": []string{dep.Database},
			}
		}
		if dep.Password != "" {
			if auth, ok := vals["auth"].(map[string]any); ok {
				auth["rootPassword"] = dep.Password
			} else {
				vals["auth"] = map[string]any{
					"rootPassword": dep.Password,
				}
			}
		}
		return vals
	default:
		return info.DefaultValues
	}
}

func renderTemplate(tmplPath, outPath string, data chartData) error {
	content, err := templateFS.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", tmplPath, err)
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", tmplPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %s: %w", tmplPath, err)
	}

	return os.WriteFile(outPath, buf.Bytes(), 0644)
}
