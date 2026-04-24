package inspect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type claudeResponse struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	IsError          bool            `json:"is_error"`
	SessionID        string          `json:"session_id"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
}

func findClaude(pathOverride string) (string, error) {
	if pathOverride != "" {
		if _, err := os.Stat(pathOverride); err == nil {
			return pathOverride, nil
		}
	}
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude CLI not found in PATH. Install: https://docs.anthropic.com/en/docs/claude-code")
	}
	return path, nil
}

type ClaudeBackend struct {
	BinaryPath string
	WorkDir    string
}

func NewClaudeBackend(workDir string) (*ClaudeBackend, error) {
	path, err := findClaude("")
	if err != nil {
		return nil, err
	}
	return &ClaudeBackend{BinaryPath: path, WorkDir: workDir}, nil
}

func (c *ClaudeBackend) Analyze(ctx context.Context, systemPrompt, userPrompt, schema string) (*AnalysisResult, string, error) {
	args := []string{
		"-p",
		"--output-format", "json",
		"--json-schema", schema,
		"--system-prompt", systemPrompt,
		"--tools", "Read,Bash(find *),Bash(cat *),Bash(ls *),Bash(head *),Bash(grep *)",
		"--dangerously-skip-permissions",
	}

	stdout, err := c.run(ctx, args, userPrompt)
	if err != nil {
		return nil, "", fmt.Errorf("claude analyze: %w", err)
	}

	return parseAnalysisResponse(stdout)
}

func (c *ClaudeBackend) Resume(ctx context.Context, sessionID, feedback, schema string) (*AnalysisResult, string, error) {
	args := []string{
		"-p",
		"--output-format", "json",
		"--json-schema", schema,
		"--resume", sessionID,
		"--tools", "Read,Bash(find *),Bash(cat *),Bash(ls *),Bash(head *),Bash(grep *)",
		"--dangerously-skip-permissions",
	}

	stdout, err := c.run(ctx, args, feedback)
	if err != nil {
		return nil, "", fmt.Errorf("claude resume: %w", err)
	}

	return parseAnalysisResponse(stdout)
}

func (c *ClaudeBackend) GeneratePlan(ctx context.Context, sessionID, schema string) (*PlanResult, error) {
	args := []string{
		"-p",
		"--output-format", "json",
		"--json-schema", schema,
		"--resume", sessionID,
		"--dangerously-skip-permissions",
	}

	stdout, err := c.run(ctx, args, BuildPlanPrompt)
	if err != nil {
		return nil, fmt.Errorf("claude generate plan: %w", err)
	}

	return parsePlanResponse(stdout)
}

func (c *ClaudeBackend) run(ctx context.Context, args []string, input string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.BinaryPath, args...)
	cmd.Dir = c.WorkDir
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

func parseAnalysisResponse(data []byte) (*AnalysisResult, string, error) {
	var resp claudeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, "", fmt.Errorf("parse claude response: %w", err)
	}
	if resp.IsError {
		return nil, "", fmt.Errorf("claude returned error: %s", resp.Result)
	}

	var result AnalysisResult
	if err := json.Unmarshal(resp.StructuredOutput, &result); err != nil {
		return nil, "", fmt.Errorf("parse structured output: %w", err)
	}

	return &result, resp.SessionID, nil
}

func parsePlanResponse(data []byte) (*PlanResult, error) {
	var resp claudeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse claude response: %w", err)
	}
	if resp.IsError {
		return nil, fmt.Errorf("claude returned error: %s", resp.Result)
	}

	var result PlanResult
	if err := json.Unmarshal(resp.StructuredOutput, &result); err != nil {
		return nil, fmt.Errorf("parse structured output: %w", err)
	}

	return &result, nil
}
