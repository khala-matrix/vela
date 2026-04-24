package inspect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Scan(projectPath string) (*ScanResult, error) {
	info, err := os.Stat(projectPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access project path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", projectPath)
	}

	r := &ScanResult{ProjectPath: projectPath}
	r.Tree = buildTree(projectPath)
	scanFiles(projectPath, r)
	classify(r)

	return r, nil
}

func buildTree(root string) string {
	var b strings.Builder
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() && (name == "node_modules" || name == ".git" || name == "__pycache__" ||
			name == "venv" || name == ".venv" || name == ".next" || name == ".vela") {
			return filepath.SkipDir
		}
		b.WriteString(rel)
		b.WriteString("\n")
		return nil
	})
	return b.String()
}

func scanFiles(root string, r *ScanResult) {
	globs := []struct {
		pattern string
		handler func(path string, r *ScanResult)
	}{
		{"package.json", handlePackageJSON},
		{"*/package.json", handlePackageJSON},
		{"requirements.txt", handleRequirements},
		{"*/requirements.txt", handleRequirements},
		{"go.mod", func(_ string, r *ScanResult) { r.HasGoMod = true }},
		{"*/go.mod", func(_ string, r *ScanResult) { r.HasGoMod = true }},
		{"Dockerfile", func(_ string, r *ScanResult) { r.HasDockerfile = true }},
		{"*/Dockerfile", func(_ string, r *ScanResult) { r.HasDockerfile = true }},
		{"docker-compose.yaml", func(_ string, r *ScanResult) { r.HasDockerCompose = true }},
		{"docker-compose.yml", func(_ string, r *ScanResult) { r.HasDockerCompose = true }},
		{"next.config.ts", func(_ string, r *ScanResult) { r.HasNextConfig = true }},
		{"*/next.config.ts", func(_ string, r *ScanResult) { r.HasNextConfig = true }},
		{"next.config.js", func(_ string, r *ScanResult) { r.HasNextConfig = true }},
		{"*/next.config.js", func(_ string, r *ScanResult) { r.HasNextConfig = true }},
		{"next.config.mjs", func(_ string, r *ScanResult) { r.HasNextConfig = true }},
		{"*/next.config.mjs", func(_ string, r *ScanResult) { r.HasNextConfig = true }},
		{"nginx.conf", func(_ string, r *ScanResult) { r.HasNginxConf = true }},
		{"*/nginx.conf", func(_ string, r *ScanResult) { r.HasNginxConf = true }},
		{".env", handleEnvFile},
	}

	for _, g := range globs {
		matches, err := filepath.Glob(filepath.Join(root, g.pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			g.handler(m, r)
		}
	}
}

func handlePackageJSON(path string, r *ScanResult) {
	r.HasPackageJSON = true
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return
	}
	if r.PackageJSONDeps == nil {
		r.PackageJSONDeps = make(map[string]string)
	}
	for k, v := range pkg.Dependencies {
		r.PackageJSONDeps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		r.PackageJSONDeps[k] = v
	}
}

func handleRequirements(path string, r *ScanResult) {
	r.HasRequirements = true
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			r.PythonDeps = append(r.PythonDeps, line)
		}
	}
}

func handleEnvFile(path string, r *ScanResult) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, _, ok := strings.Cut(line, "="); ok {
			r.EnvVars = append(r.EnvVars, strings.TrimSpace(k))
		}
	}
}

func classify(r *ScanResult) {
	hasNext := r.HasPackageJSON && r.PackageJSONDeps["next"] != ""
	hasFastAPI := hasPythonDep(r, "fastapi")
	hasPgDep := hasPythonDep(r, "asyncpg") || hasPythonDep(r, "psycopg2") ||
		hasPythonDep(r, "sqlalchemy") || containsEnv(r, "DATABASE_URL")
	hasNginxOrStatic := r.HasNginxConf

	switch {
	case hasNext && hasFastAPI && hasPgDep:
		r.DetectedStack = StackNextjsFastapiPg
		r.DetectedReason = "Found Next.js in package.json, FastAPI in requirements.txt, and PostgreSQL dependency"
	case hasNext && hasFastAPI:
		r.DetectedStack = StackNextjsFastapi
		r.DetectedReason = "Found Next.js in package.json and FastAPI in requirements.txt"
	case hasNginxOrStatic:
		r.DetectedStack = StackStaticSite
		r.DetectedReason = "Found nginx.conf for static site serving"
	default:
		r.DetectedStack = StackUnsupported
		r.DetectedReason = "Project does not match any supported vela template"
	}
}

func hasPythonDep(r *ScanResult, name string) bool {
	for _, dep := range r.PythonDeps {
		lower := strings.ToLower(dep)
		if strings.HasPrefix(lower, name) {
			return true
		}
	}
	return false
}

func containsEnv(r *ScanResult, key string) bool {
	for _, k := range r.EnvVars {
		if k == key {
			return true
		}
	}
	return false
}
