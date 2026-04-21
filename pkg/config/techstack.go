package config

import (
	"crypto/rand"
	"encoding/hex"
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
	Name         string                 `yaml:"name,omitempty"`
	App          App                    `yaml:"app,omitempty"`
	Services     []Service              `yaml:"services,omitempty"`
	Dependencies map[string]*Dependency `yaml:"dependencies,omitempty"`
}

type Service struct {
	Name      string    `yaml:"name"`
	Image     string    `yaml:"image"`
	Port      int       `yaml:"port"`
	Replicas  int       `yaml:"replicas,omitempty"`
	Env       []EnvVar  `yaml:"env,omitempty"`
	Resources Resources `yaml:"resources,omitempty"`
	Ingress   *Ingress  `yaml:"ingress,omitempty"`
}

type Ingress struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host,omitempty"`
	Path    string `yaml:"path,omitempty"`
}

// App is the legacy single-service format. Kept for backward compatibility.
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

func (ts *TechStack) IsMultiService() bool {
	return len(ts.Services) > 0
}

func (ts *TechStack) ProjectName() string {
	if ts.Name != "" {
		return ts.Name
	}
	return ts.App.Name
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

	// Legacy single-service format: convert app → services
	if ts.App.Image != "" && len(ts.Services) == 0 {
		if ts.Name == "" {
			ts.Name = ts.App.Name
		}
		ts.Services = []Service{appToService(ts.App)}
	}

	if err := validate(&ts); err != nil {
		return nil, err
	}

	applyDefaults(&ts)

	return &ts, nil
}

func appToService(a App) Service {
	return Service{
		Name:      a.Name,
		Image:     a.Image,
		Port:      a.Port,
		Replicas:  a.Replicas,
		Env:       a.Env,
		Resources: a.Resources,
	}
}

func validate(ts *TechStack) error {
	if ts.Name == "" && len(ts.Services) > 0 {
		return fmt.Errorf("missing required field: name")
	}

	// Legacy single-service path
	if len(ts.Services) == 0 {
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
	}

	for i, svc := range ts.Services {
		var missing []string
		if svc.Name == "" {
			missing = append(missing, fmt.Sprintf("services[%d].name", i))
		}
		if svc.Image == "" {
			missing = append(missing, fmt.Sprintf("services[%d].image", i))
		}
		if svc.Port == 0 {
			missing = append(missing, fmt.Sprintf("services[%d].port", i))
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
		}
	}

	names := make(map[string]bool)
	for _, svc := range ts.Services {
		if names[svc.Name] {
			return fmt.Errorf("duplicate service name: %q", svc.Name)
		}
		names[svc.Name] = true
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
	// Legacy format defaults
	if ts.App.Image != "" {
		if ts.App.Replicas == 0 {
			ts.App.Replicas = 1
		}
		if ts.App.Resources.CPU == "" {
			ts.App.Resources.CPU = "250m"
		}
		if ts.App.Resources.Memory == "" {
			ts.App.Resources.Memory = "256Mi"
		}
	}

	for i := range ts.Services {
		if ts.Services[i].Replicas == 0 {
			ts.Services[i].Replicas = 1
		}
		if ts.Services[i].Resources.CPU == "" {
			ts.Services[i].Resources.CPU = "250m"
		}
		if ts.Services[i].Resources.Memory == "" {
			ts.Services[i].Resources.Memory = "256Mi"
		}
	}

	for i := range ts.Services {
		if ts.Services[i].Ingress != nil && ts.Services[i].Ingress.Enabled {
			if ts.Services[i].Ingress.Path == "" {
				ts.Services[i].Ingress.Path = "/"
			}
			if ts.Services[i].Ingress.Host == "" {
				ts.Services[i].Ingress.Host = ts.Services[i].Name + "-" + randomPrefix() + ".nip.io"
			}
		}
	}

	for name, dep := range ts.Dependencies {
		if dep.Storage == "" {
			dep.Storage = "1Gi"
		}
		if dep.Database == "" && (name == "mysql" || name == "postgresql" || name == "mongodb") {
			dep.Database = ts.ProjectName()
		}
	}
}

func randomPrefix() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func SupportedDependencies() []string {
	deps := make([]string, 0, len(supportedDependencies))
	for k := range supportedDependencies {
		deps = append(deps, k)
	}
	return deps
}
