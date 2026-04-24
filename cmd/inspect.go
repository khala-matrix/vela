package cmd

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mars/vela/pkg/inspect"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [path]",
	Short: "Analyze an existing project for vela deployment readiness",
	Long: `Scan a project directory, detect its tech stack, and use Claude Code
to generate tech-stack.yaml, build.sh, and an implementation plan.

Requires 'claude' CLI installed and authenticated.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	if isJSON() {
		return fmt.Errorf("inspect does not support --output json; it requires interactive TUI")
	}

	p := tea.NewProgram(newInspectModel(projectPath, namespace), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return err
	}
	final := m.(inspectModel)
	if final.err != nil {
		return final.err
	}
	return nil
}

// --- TUI phases ---

type inspectPhase int

const (
	phaseScanning inspectPhase = iota
	phaseUnsupported
	phaseAnalyzing
	phaseReview
	phaseFeedbackInput
	phaseValidating
	phaseValidationFailed
	phaseGeneratingPlan
	phaseDone
)

// --- Messages ---

type scanDoneMsg struct {
	result *inspect.ScanResult
	err    error
}
type analyzeDoneMsg struct {
	result    *inspect.AnalysisResult
	sessionID string
	err       error
}
type validateDoneMsg struct {
	errs []inspect.ValidationError
}
type planDoneMsg struct {
	result *inspect.PlanResult
	err    error
}
type artifactsDoneMsg struct {
	err error
}

// --- Model ---

type inspectModel struct {
	phase       inspectPhase
	projectPath string
	namespace   string
	width       int
	height      int

	scan      *inspect.ScanResult
	analysis  *inspect.AnalysisResult
	sessionID string
	plan      *inspect.PlanResult
	valErrs   []inspect.ValidationError

	feedback     string
	reviewTab    int
	scrollOffset int

	backend *inspect.ClaudeBackend
	err     error
}

var (
	iTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	iDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	iHighlight = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	iSuccess   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	iError     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	iWarn      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

var reviewTabs = []string{"Summary", "tech-stack.yaml", "build.sh", "Changes"}

func newInspectModel(projectPath, ns string) inspectModel {
	return inspectModel{
		phase:       phaseScanning,
		projectPath: projectPath,
		namespace:   ns,
	}
}

func (m inspectModel) Init() tea.Cmd {
	return func() tea.Msg {
		result, err := inspect.Scan(m.projectPath)
		return scanDoneMsg{result: result, err: err}
	}
}

func (m inspectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case scanDoneMsg:
		return m.handleScanDone(msg)
	case analyzeDoneMsg:
		return m.handleAnalyzeDone(msg)
	case validateDoneMsg:
		return m.handleValidateDone(msg)
	case planDoneMsg:
		return m.handlePlanDone(msg)
	case artifactsDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.phase = phaseDone
		return m, nil
	}

	return m, nil
}

func (m inspectModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseUnsupported:
		if msg.String() == "q" || msg.String() == "enter" {
			return m, tea.Quit
		}

	case phaseReview:
		switch msg.String() {
		case "tab":
			m.reviewTab = (m.reviewTab + 1) % len(reviewTabs)
			m.scrollOffset = 0
		case "shift+tab":
			m.reviewTab = (m.reviewTab - 1 + len(reviewTabs)) % len(reviewTabs)
			m.scrollOffset = 0
		case "j", "down":
			m.scrollOffset++
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "f":
			m.phase = phaseFeedbackInput
			m.feedback = ""
		case "a":
			m.phase = phaseValidating
			return m, m.cmdValidate()
		case "q":
			return m, tea.Quit
		}

	case phaseFeedbackInput:
		switch msg.String() {
		case "enter":
			if m.feedback != "" {
				m.phase = phaseAnalyzing
				return m, m.cmdResume(m.feedback)
			}
		case "esc":
			m.phase = phaseReview
		case "backspace":
			if len(m.feedback) > 0 {
				m.feedback = m.feedback[:len(m.feedback)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.feedback += msg.String()
			}
		}

	case phaseValidationFailed:
		switch msg.String() {
		case "enter":
			m.phase = phaseAnalyzing
			fb := "Validation failed. Fix these errors and regenerate:\n"
			for _, e := range m.valErrs {
				fb += fmt.Sprintf("- %s: %s\n", e.Field, e.Message)
			}
			return m, m.cmdResume(fb)
		case "q":
			return m, tea.Quit
		}

	case phaseDone:
		if msg.String() == "q" || msg.String() == "enter" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m inspectModel) handleScanDone(msg scanDoneMsg) (inspectModel, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, tea.Quit
	}
	m.scan = msg.result

	if m.scan.DetectedStack == inspect.StackUnsupported {
		m.phase = phaseUnsupported
		return m, nil
	}

	backend, err := inspect.NewClaudeBackend(m.projectPath)
	if err != nil {
		m.err = err
		return m, tea.Quit
	}
	m.backend = backend
	m.phase = phaseAnalyzing
	return m, m.cmdAnalyze()
}

func (m inspectModel) handleAnalyzeDone(msg analyzeDoneMsg) (inspectModel, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, tea.Quit
	}
	m.analysis = msg.result
	m.sessionID = msg.sessionID
	m.phase = phaseReview
	m.reviewTab = 0
	m.scrollOffset = 0
	return m, nil
}

func (m inspectModel) handleValidateDone(msg validateDoneMsg) (inspectModel, tea.Cmd) {
	if len(msg.errs) > 0 {
		m.valErrs = msg.errs
		m.phase = phaseValidationFailed
		return m, nil
	}
	m.phase = phaseGeneratingPlan
	return m, m.cmdGeneratePlan()
}

func (m inspectModel) handlePlanDone(msg planDoneMsg) (inspectModel, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, tea.Quit
	}
	m.plan = msg.result
	return m, func() tea.Msg {
		err := inspect.WriteArtifacts(m.projectPath, m.analysis, m.plan)
		return artifactsDoneMsg{err: err}
	}
}

// --- Commands ---

func (m inspectModel) cmdAnalyze() tea.Cmd {
	return func() tea.Msg {
		guide, err := RenderGuide(string(m.scan.DetectedStack), guideData{
			Name:            "PROJECT_NAME",
			Namespace:       m.namespace,
			Registry:        defaultRegistry,
			Domain:          defaultDomain,
			BaseRegistry:    defaultBaseRegistry,
			DBImageRegistry: defaultDBImageRegistry,
			BuildTool:       "docker",
			BuildCmd:        "build",
		})
		if err != nil {
			return analyzeDoneMsg{err: err}
		}

		systemPrompt, err := inspect.BuildSystemPrompt(guide)
		if err != nil {
			return analyzeDoneMsg{err: err}
		}

		userPrompt := inspect.BuildAnalysisPrompt(m.scan, m.namespace, defaultRegistry, defaultDomain)
		result, sessionID, err := m.backend.Analyze(
			context.Background(), systemPrompt, userPrompt, inspect.AnalysisSchema,
		)
		return analyzeDoneMsg{result: result, sessionID: sessionID, err: err}
	}
}

func (m inspectModel) cmdResume(feedback string) tea.Cmd {
	return func() tea.Msg {
		result, sessionID, err := m.backend.Resume(
			context.Background(), m.sessionID, feedback, inspect.AnalysisSchema,
		)
		return analyzeDoneMsg{result: result, sessionID: sessionID, err: err}
	}
}

func (m inspectModel) cmdValidate() tea.Cmd {
	return func() tea.Msg {
		errs := inspect.Validate(m.analysis)
		return validateDoneMsg{errs: errs}
	}
}

func (m inspectModel) cmdGeneratePlan() tea.Cmd {
	return func() tea.Msg {
		result, err := m.backend.GeneratePlan(
			context.Background(), m.sessionID, inspect.PlanSchema,
		)
		return planDoneMsg{result: result, err: err}
	}
}

// --- View ---

func (m inspectModel) View() string {
	var b strings.Builder

	b.WriteString(iTitle.Render("vela inspect"))
	b.WriteString("\n\n")

	switch m.phase {
	case phaseScanning:
		fmt.Fprintf(&b, "  Scanning %s ...\n", m.projectPath)

	case phaseUnsupported:
		b.WriteString(iError.Render("  Unsupported stack"))
		b.WriteString("\n\n")
		fmt.Fprintf(&b, "  Detected: %s\n", m.scan.DetectedReason)
		b.WriteString("\n  Vela currently supports:\n")
		for _, s := range inspect.SupportedStacks {
			fmt.Fprintf(&b, "    - %s\n", s)
		}
		b.WriteString(iDim.Render("\n  press q to exit"))

	case phaseAnalyzing:
		fmt.Fprintf(&b, "  Stack: %s\n", iHighlight.Render(string(m.scan.DetectedStack)))
		fmt.Fprintf(&b, "  AI backend: claude\n\n")
		b.WriteString("  Analyzing project...\n")
		b.WriteString(iDim.Render("  (Claude is reading your project files)"))

	case phaseReview:
		m.viewReview(&b)

	case phaseFeedbackInput:
		m.viewReview(&b)
		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s %s\n", iHighlight.Render("Feedback:"), m.feedback)
		b.WriteString(iDim.Render("  enter send | esc cancel"))

	case phaseValidating:
		b.WriteString("  Validating artifacts...\n")

	case phaseValidationFailed:
		b.WriteString(iWarn.Render("  Validation failed"))
		b.WriteString("\n\n")
		for _, e := range m.valErrs {
			fmt.Fprintf(&b, "  - %s: %s\n", iError.Render(e.Field), e.Message)
		}
		b.WriteString(iDim.Render("\n  enter auto-fix (send errors to Claude) | q quit"))

	case phaseGeneratingPlan:
		b.WriteString("  Generating implementation plan...\n")

	case phaseDone:
		b.WriteString(iSuccess.Render("  Artifacts saved to .vela/inspect-report/"))
		b.WriteString("\n\n")
		b.WriteString("  Next steps:\n")
		fmt.Fprintf(&b, "    cp .vela/inspect-report/tech-stack.yaml .\n")
		fmt.Fprintf(&b, "    cp .vela/inspect-report/build.sh .\n")
		fmt.Fprintf(&b, "    claude -p < .vela/inspect-report/plan.md\n")
		fmt.Fprintf(&b, "    vela deploy\n")
		b.WriteString(iDim.Render("\n  press q to exit"))
	}

	b.WriteString("\n")
	return b.String()
}

func (m inspectModel) viewReview(b *strings.Builder) {
	for i, tab := range reviewTabs {
		if i == m.reviewTab {
			fmt.Fprintf(b, "  %s", iHighlight.Render("["+tab+"]"))
		} else {
			fmt.Fprintf(b, "  %s", iDim.Render(" "+tab+" "))
		}
	}
	b.WriteString("\n\n")

	maxLines := m.height - 12
	if maxLines < 10 {
		maxLines = 20
	}

	var content string
	switch m.reviewTab {
	case 0:
		content = m.analysis.Summary
		if len(m.analysis.Issues) > 0 {
			content += "\n\nIssues:\n"
			for _, issue := range m.analysis.Issues {
				prefix := "-"
				if issue.Severity == "warning" {
					prefix = "!"
				}
				content += fmt.Sprintf("  %s %s\n", prefix, issue.Message)
			}
		}
	case 1:
		content = m.analysis.TechStackYAML
	case 2:
		content = m.analysis.BuildSH
	case 3:
		for _, ch := range m.analysis.FileChanges {
			content += fmt.Sprintf("  %s — %s (%s)\n", ch.File, ch.Description, ch.Action)
			if ch.CurrentSnippet != "" {
				content += fmt.Sprintf("  current:   %s\n", truncate(ch.CurrentSnippet, 80))
			}
			content += fmt.Sprintf("  suggested: %s\n\n", truncate(ch.SuggestedSnippet, 80))
		}
		if len(m.analysis.FileChanges) == 0 {
			content = "No file changes needed."
		}
	}

	lines := strings.Split(content, "\n")
	start := m.scrollOffset
	if start > len(lines) {
		start = len(lines)
	}
	end := start + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	for _, line := range lines[start:end] {
		fmt.Fprintf(b, "  %s\n", line)
	}
	if end < len(lines) {
		fmt.Fprintf(b, "  %s\n", iDim.Render(fmt.Sprintf("... %d more lines (j/k to scroll)", len(lines)-end)))
	}

	b.WriteString("\n")
	b.WriteString(iDim.Render("  tab sections | f feedback | a accept | j/k scroll | q quit"))
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
