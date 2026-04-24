package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

var AnalysisSchema = mustMarshal(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"summary":         map[string]string{"type": "string"},
		"tech_stack_yaml": map[string]string{"type": "string"},
		"build_sh":        map[string]string{"type": "string"},
		"file_changes": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":              map[string]string{"type": "string"},
					"action":            map[string]string{"type": "string"},
					"description":       map[string]string{"type": "string"},
					"current_snippet":   map[string]string{"type": "string"},
					"suggested_snippet": map[string]string{"type": "string"},
				},
				"required": []string{"file", "action", "description", "suggested_snippet"},
			},
		},
		"issues": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"severity": map[string]string{"type": "string"},
					"message":  map[string]string{"type": "string"},
				},
				"required": []string{"severity", "message"},
			},
		},
	},
	"required": []string{"summary", "tech_stack_yaml", "build_sh", "file_changes", "issues"},
})

var PlanSchema = mustMarshal(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"plan": map[string]string{"type": "string"},
	},
	"required": []string{"plan"},
})

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

const systemPromptTemplate = `You are a deployment analyzer for vela, a CLI that deploys applications to k3s clusters with Traefik ingress controller.

Your task: analyze the project at the given path and generate deployment artifacts that are GUARANTEED to work with vela deploy.

CRITICAL CONSTRAINTS — these are vela invariants, you MUST follow them:
1. tech_stack_yaml must be valid YAML that conforms to the vela tech-stack.yaml schema
2. Ingress paths must follow the pattern: /<namespace>/<appname> for frontend, /<namespace>/<appname>/api for backend
3. stripPrefix must be false — Traefik passes the full path through
4. Frontend (Next.js) needs basePath matching the ingress path
5. All fetch() calls need the basePath prefix (basePath does NOT apply to fetch)
6. Backend (FastAPI) needs APIRouter prefix matching the ingress path
7. Dockerfiles must use USER 1000 for the runtime stage
8. build.sh must support REGISTRY, TAG, PLATFORM env vars
9. Dependencies only: postgresql, mysql, redis, mongodb
10. Every service image in tech_stack_yaml must have a matching build+push in build_sh

Below is the authoritative setup guide for this stack type. Your generated tech_stack_yaml and build_sh MUST conform to these patterns.

=== SETUP GUIDE ===
{{.Guide}}
=== END GUIDE ===

You have Read and Bash tools. Read whatever project files you need to produce an accurate analysis. Examine source code, configs, and existing Dockerfiles.

Return your analysis as structured JSON. tech_stack_yaml and build_sh must be COMPLETE file contents, not snippets.`

func BuildSystemPrompt(guide string) (string, error) {
	tmpl, err := template.New("system").Parse(systemPromptTemplate)
	if err != nil {
		return "", fmt.Errorf("parse system prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"Guide": guide}); err != nil {
		return "", fmt.Errorf("render system prompt: %w", err)
	}
	return buf.String(), nil
}

func BuildAnalysisPrompt(scan *ScanResult, namespace, registry, domain string) string {
	return fmt.Sprintf(`Analyze the project at: %s

Detected stack: %s (%s)
Namespace: %s
Registry: %s
Domain: %s

Directory tree:
%s

Read the project files you need and generate the complete deployment artifacts.`,
		scan.ProjectPath, scan.DetectedStack, scan.DetectedReason,
		namespace, registry, domain, scan.Tree)
}

const BuildPlanPrompt = `Based on the analysis you just completed, generate an implementation plan in markdown.

The plan must be:
1. Human-readable — a developer can follow it step by step
2. AI-executable — another Claude or Codex instance can execute it via "claude -p < plan.md"

Format each required change as a numbered task with:
- File path
- What to change and why
- Current code snippet (if modifying existing file)
- Suggested code snippet (complete, copy-pasteable)

Start with instructions to copy tech-stack.yaml and build.sh from .vela/inspect-report/, then list code changes.
End with "vela deploy" as the final step.

Return the plan as a single markdown string in the "plan" field.`
