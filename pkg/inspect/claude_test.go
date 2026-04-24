package inspect

import (
	"testing"
)

func TestParseAnalysisResponse_Valid(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"session_id": "abc-123",
		"result": "",
		"structured_output": {
			"summary": "Found Next.js + FastAPI project",
			"tech_stack_yaml": "name: test\nservices:\n  - name: test-backend\n    image: r/test-backend:latest\n    port: 8000",
			"build_sh": "#!/usr/bin/env bash\necho hi",
			"file_changes": [],
			"issues": []
		}
	}`

	result, sessionID, err := parseAnalysisResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parseAnalysisResponse failed: %v", err)
	}
	if sessionID != "abc-123" {
		t.Errorf("expected session_id abc-123, got %s", sessionID)
	}
	if result.Summary != "Found Next.js + FastAPI project" {
		t.Errorf("unexpected summary: %s", result.Summary)
	}
	if result.TechStackYAML == "" {
		t.Error("expected non-empty tech_stack_yaml")
	}
}

func TestParseAnalysisResponse_Error(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "error",
		"is_error": true,
		"result": "something went wrong"
	}`

	_, _, err := parseAnalysisResponse([]byte(raw))
	if err == nil {
		t.Fatal("expected error for error response")
	}
}

func TestParsePlanResponse_Valid(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"session_id": "abc-123",
		"structured_output": {
			"plan": "# Implementation Plan\n\n## Step 1\n..."
		}
	}`

	result, err := parsePlanResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parsePlanResponse failed: %v", err)
	}
	if result.Plan == "" {
		t.Error("expected non-empty plan")
	}
}
