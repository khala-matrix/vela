package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var supportedDependencies = map[string]bool{
	"mysql":      true,
	"postgresql": true,
	"redis":      true,
	"mongodb":    true,
}

type TechStack struct {
	App          App                    `yaml:"app"`
	Dependencies map[string]*Dependency `yaml:"dependencies,omitempty"`
}

type App struct {
	Name      string    `yaml:"name"`
	Image     string    `yaml:"image"`
	Port      int       `yaml:"port"`
	Replicas  int       `yaml:"replicas,omitempty"`
	Env       []EnvVar  `yaml:"env,omitempty"`
	Resources Resources `yaml:"resources,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Resources struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type Dependency struct {
	Version  string `yaml:"version"`
	Storage  string `yaml:"storage,omitempty"`
	Password string `yaml:"password,omitempty"`
	Database string `yaml:"database,omitempty"`
}

func Parse(path string) (*TechStack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tech-stack file: %w", err)
	}

	var ts TechStack
	if err := yaml.Unmarshal(data, &ts); err != nil {
		return nil, fmt.Errorf("parse tech-stack yaml: %w", err)
	}

	if err := validate(&ts); err != nil {
		return nil, err
	}

	applyDefaults(&ts)

	return &ts, nil
}

func validate(ts *TechStack) error {
	var missing []string
	if ts.App.Name == "" {
		missing = append(missing, "app.name")
	}
	if ts.App.Image == "" {
		missing = append(missing, "app.image")
	}
	if ts.App.Port == 0 {
		missing = append(missing, "app.port")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	for name, dep := range ts.Dependencies {
		if !supportedDependencies[name] {
			supported := make([]string, 0, len(supportedDependencies))
			for k := range supportedDependencies {
				supported = append(supported, k)
			}
			return fmt.Errorf("unsupported dependency %q, supported: %s", name, strings.Join(supported, ", "))
		}
		if dep.Version == "" {
			return fmt.Errorf("dependency %q missing required field: version", name)
		}
	}

	return nil
}

func applyDefaults(ts *TechStack) {
	if ts.App.Replicas == 0 {
		ts.App.Replicas = 1
	}
	if ts.App.Resources.CPU == "" {
		ts.App.Resources.CPU = "250m"
	}
	if ts.App.Resources.Memory == "" {
		ts.App.Resources.Memory = "256Mi"
	}

	for name, dep := range ts.Dependencies {
		if dep.Storage == "" {
			dep.Storage = "1Gi"
		}
		if dep.Database == "" && (name == "mysql" || name == "postgresql" || name == "mongodb") {
			dep.Database = ts.App.Name
		}
	}
}

func SupportedDependencies() []string {
	deps := make([]string, 0, len(supportedDependencies))
	for k := range supportedDependencies {
		deps = append(deps, k)
	}
	return deps
}
