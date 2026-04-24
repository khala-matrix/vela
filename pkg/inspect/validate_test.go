package inspect

import (
	"testing"
)

func TestValidate_ValidResult(t *testing.T) {
	result := &AnalysisResult{
		TechStackYAML: `name: test-app
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
`,
		BuildSH: `#!/usr/bin/env bash
set -euo pipefail
REGISTRY="${REGISTRY:-registry.example.com}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"
docker build --platform "${PLATFORM}" -t "${REGISTRY}/test-app-backend:${TAG}" ./backend
docker build --platform "${PLATFORM}" -t "${REGISTRY}/test-app-frontend:${TAG}" ./frontend
docker push "${REGISTRY}/test-app-backend:${TAG}"
docker push "${REGISTRY}/test-app-frontend:${TAG}"
`,
	}

	errs := Validate(result)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidate_InvalidYAML(t *testing.T) {
	result := &AnalysisResult{
		TechStackYAML: `name: test
services:
  - name: broken
    port: 8000
`,
		BuildSH: "#!/usr/bin/env bash\necho hi\n",
	}

	errs := Validate(result)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for missing image")
	}
	found := false
	for _, e := range errs {
		if e.Field == "tech_stack_yaml" {
			found = true
		}
	}
	if !found {
		t.Error("expected a tech_stack_yaml validation error")
	}
}

func TestValidate_BadBashSyntax(t *testing.T) {
	result := &AnalysisResult{
		TechStackYAML: `name: test-app
services:
  - name: test-app-web
    image: reg/test-app-web:latest
    port: 3000
`,
		BuildSH: "#!/usr/bin/env bash\nif [[ ; then\n",
	}

	errs := Validate(result)
	found := false
	for _, e := range errs {
		if e.Field == "build_sh" {
			found = true
		}
	}
	if !found {
		t.Error("expected a build_sh syntax validation error")
	}
}

func TestValidate_MissingImageInBuildSH(t *testing.T) {
	result := &AnalysisResult{
		TechStackYAML: `name: test-app
services:
  - name: test-app-backend
    image: registry.example.com/test-app-backend:latest
    port: 8000
  - name: test-app-frontend
    image: registry.example.com/test-app-frontend:latest
    port: 3000
`,
		BuildSH: `#!/usr/bin/env bash
set -euo pipefail
docker build -t registry.example.com/test-app-backend:latest ./backend
docker push registry.example.com/test-app-backend:latest
`,
	}

	errs := Validate(result)
	found := false
	for _, e := range errs {
		if e.Field == "build_sh_images" {
			found = true
		}
	}
	if !found {
		t.Error("expected a build_sh_images error for missing frontend build")
	}
}

func TestValidate_EmptyBuildSH(t *testing.T) {
	result := &AnalysisResult{
		TechStackYAML: `name: test-app
services:
  - name: test-app-web
    image: reg/test-app-web:latest
    port: 3000
`,
		BuildSH: "",
	}

	errs := Validate(result)
	found := false
	for _, e := range errs {
		if e.Field == "build_sh" {
			found = true
		}
	}
	if !found {
		t.Error("expected a build_sh empty validation error")
	}
}
