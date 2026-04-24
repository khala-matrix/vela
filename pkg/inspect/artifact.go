package inspect

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteArtifacts(projectPath string, analysis *AnalysisResult, plan *PlanResult) error {
	reportDir := filepath.Join(projectPath, ".vela", "inspect-report")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	files := []struct {
		name    string
		content string
		perm    os.FileMode
	}{
		{"tech-stack.yaml", analysis.TechStackYAML, 0644},
		{"build.sh", analysis.BuildSH, 0755},
		{"plan.md", plan.Plan, 0644},
	}

	for _, f := range files {
		path := filepath.Join(reportDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), f.perm); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
	}

	return nil
}
