package inspect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mars/vela/pkg/config"
)

func Validate(result *AnalysisResult) []ValidationError {
	var errs []ValidationError

	if result.TechStackYAML == "" {
		errs = append(errs, ValidationError{Field: "tech_stack_yaml", Message: "empty tech-stack.yaml"})
	} else {
		ts, err := config.ParseBytes([]byte(result.TechStackYAML))
		if err != nil {
			errs = append(errs, ValidationError{Field: "tech_stack_yaml", Message: err.Error()})
		} else {
			errs = append(errs, checkImageConsistency(ts, result.BuildSH)...)
		}
	}

	if result.BuildSH == "" {
		errs = append(errs, ValidationError{Field: "build_sh", Message: "empty build.sh"})
	} else {
		if err := checkBashSyntax(result.BuildSH); err != nil {
			errs = append(errs, ValidationError{Field: "build_sh", Message: "syntax error: " + err.Error()})
		}
	}

	return errs
}

func checkBashSyntax(script string) error {
	tmpDir, err := os.MkdirTemp("", "vela-validate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "build.sh")
	if err := os.WriteFile(path, []byte(script), 0644); err != nil {
		return err
	}

	cmd := exec.Command("bash", "-n", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func checkImageConsistency(ts *config.TechStack, buildSH string) []ValidationError {
	var errs []ValidationError
	for _, svc := range ts.Services {
		imageName := extractImageBase(svc.Image)
		if imageName == "" {
			continue
		}
		if !strings.Contains(buildSH, imageName) {
			errs = append(errs, ValidationError{
				Field:   "build_sh_images",
				Message: fmt.Sprintf("service %q image %q not found in build.sh", svc.Name, svc.Image),
			})
		}
	}
	return errs
}

func extractImageBase(image string) string {
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		image = image[:idx]
	}
	if idx := strings.LastIndex(image, "/"); idx != -1 {
		image = image[idx+1:]
	}
	image = strings.TrimPrefix(image, "${REGISTRY}/")
	return image
}
