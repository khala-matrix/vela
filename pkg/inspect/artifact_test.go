package inspect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteArtifacts(t *testing.T) {
	dir := t.TempDir()

	analysis := &AnalysisResult{
		TechStackYAML: "name: test-app\nservices:\n  - name: web\n    image: img:latest\n    port: 3000\n",
		BuildSH:       "#!/usr/bin/env bash\nset -euo pipefail\necho build\n",
	}
	plan := &PlanResult{
		Plan: "# Implementation Plan\n\n## Step 1: Copy files\n",
	}

	err := WriteArtifacts(dir, analysis, plan)
	if err != nil {
		t.Fatalf("WriteArtifacts failed: %v", err)
	}

	reportDir := filepath.Join(dir, ".vela", "inspect-report")

	ts, err := os.ReadFile(filepath.Join(reportDir, "tech-stack.yaml"))
	if err != nil {
		t.Fatalf("tech-stack.yaml not written: %v", err)
	}
	if string(ts) != analysis.TechStackYAML {
		t.Errorf("tech-stack.yaml content mismatch")
	}

	buildPath := filepath.Join(reportDir, "build.sh")
	bs, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatalf("build.sh not written: %v", err)
	}
	if string(bs) != analysis.BuildSH {
		t.Errorf("build.sh content mismatch")
	}
	info, _ := os.Stat(buildPath)
	if info.Mode().Perm()&0111 == 0 {
		t.Error("build.sh should be executable")
	}

	pm, err := os.ReadFile(filepath.Join(reportDir, "plan.md"))
	if err != nil {
		t.Fatalf("plan.md not written: %v", err)
	}
	if string(pm) != plan.Plan {
		t.Errorf("plan.md content mismatch")
	}
}
