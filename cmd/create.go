package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/scaffold"
	"github.com/spf13/cobra"
)

var (
	createTemplate     string
	createRegistry     string
	createDomain       string
	createBaseRegistry string
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new project from a template",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createTemplate, "template", "t", "", "template ID (e.g. nextjs-fastapi, static-site)")
	createCmd.Flags().StringVar(&createRegistry, "registry", "harbor.cn.svc.corpintra.net/sandboxcoder", "image registry")
	createCmd.Flags().StringVar(&createDomain, "domain", "devbox.ittz-tech-platform.cn.svc.corpintra.net", "ingress domain")
	createCmd.Flags().StringVar(&createBaseRegistry, "base-registry", "harbor.cn.svc.corpintra.net/baselibrary", "base image registry")
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	if name != "" && createTemplate != "" && createRegistry != "" && createDomain != "" {
		return generateProject(cmd, name, createTemplate, createRegistry, createDomain, createBaseRegistry)
	}

	p := tea.NewProgram(newCreateModel(name))
	m, err := p.Run()
	if err != nil {
		return err
	}

	final := m.(createModel)
	if final.cancelled {
		fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
		return nil
	}

	tmpl := scaffold.Templates[final.templateIdx]
	return generateProject(cmd, final.inputs[0], tmpl.ID, final.inputs[1], final.inputs[2], final.inputs[3])
}

func generateProject(cmd *cobra.Command, name, templateID, registry, domain, baseRegistry string) error {
	outDir := name

	if _, err := os.Stat(outDir); err == nil {
		return fmt.Errorf("directory %q already exists", outDir)
	}

	var tmpl *scaffold.Template
	for i := range scaffold.Templates {
		if scaffold.Templates[i].ID == templateID {
			tmpl = &scaffold.Templates[i]
			break
		}
	}
	if tmpl == nil {
		ids := make([]string, len(scaffold.Templates))
		for i, t := range scaffold.Templates {
			ids[i] = t.ID
		}
		return fmt.Errorf("unknown template %q, available: %s", templateID, strings.Join(ids, ", "))
	}

	ns := cmd.Flag("namespace").Value.String()

	params := scaffold.Params{
		Name:         name,
		Namespace:    ns,
		Registry:     registry,
		Domain:       domain,
		BaseRegistry: baseRegistry,
	}

	if err := scaffold.RenderSkeleton(templateID, params, outDir); err != nil {
		return fmt.Errorf("generate skeleton: %w", err)
	}
	if err := project.Init(outDir, name, ns); err != nil {
		os.RemoveAll(outDir)
		return fmt.Errorf("init project: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created project %s/\n\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "Next steps:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  cd %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  ./build.sh        # build & push images\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  vela deploy       # deploy to cluster\n")
	return nil
}

// --- TUI Model ---

type createStep int

const (
	createStepTemplate createStep = iota
	createStepInput
	createStepConfirm
)

type createModel struct {
	step        createStep
	templateIdx int
	inputIdx    int
	inputs      [4]string
	cancelled   bool
	width       int
}

var createInputLabels = [4]string{"Project name", "Image registry", "Ingress domain", "Base image registry"}
var createInputPlaceholders = [4]string{"my-app", "harbor.cn.svc.corpintra.net/sandboxcoder", "devbox.ittz-tech-platform.cn.svc.corpintra.net", "harbor.cn.svc.corpintra.net/baselibrary"}

var (
	cTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	cSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	cDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	cInputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
)

func newCreateModel(name string) createModel {
	m := createModel{}
	if name != "" {
		m.inputs[0] = name
	}
	return m
}

func (m createModel) Init() tea.Cmd {
	return nil
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "q":
			if m.step == createStepTemplate {
				m.cancelled = true
				return m, tea.Quit
			}
		case "esc":
			if m.step == createStepInput && m.inputIdx > 0 {
				m.inputIdx--
				return m, nil
			}
			if m.step == createStepInput && m.inputIdx == 0 {
				m.step = createStepTemplate
				return m, nil
			}
			if m.step == createStepConfirm {
				m.step = createStepInput
				m.inputIdx = 3
				return m, nil
			}
		}
	}

	switch m.step {
	case createStepTemplate:
		return m.updateTemplate(msg)
	case createStepInput:
		return m.updateInput(msg)
	case createStepConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

func (m createModel) updateTemplate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.templateIdx > 0 {
				m.templateIdx--
			}
		case "down", "j":
			if m.templateIdx < len(scaffold.Templates)-1 {
				m.templateIdx++
			}
		case "enter":
			m.step = createStepInput
		}
	}
	return m, nil
}

func (m createModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter":
			if m.inputs[m.inputIdx] == "" {
				m.inputs[m.inputIdx] = createInputPlaceholders[m.inputIdx]
			}
			if m.inputIdx < 3 {
				m.inputIdx++
			} else {
				m.step = createStepConfirm
			}
		case "backspace":
			if len(m.inputs[m.inputIdx]) > 0 {
				m.inputs[m.inputIdx] = m.inputs[m.inputIdx][:len(m.inputs[m.inputIdx])-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.inputs[m.inputIdx] += msg.String()
			}
		}
	}
	return m, nil
}

func (m createModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter", "y":
			return m, tea.Quit
		case "n":
			m.step = createStepInput
			m.inputIdx = 0
			m.inputs = [4]string{}
		}
	}
	return m, nil
}

func (m createModel) View() string {
	var b strings.Builder

	b.WriteString(cTitleStyle.Render("vela create"))
	b.WriteString("\n\n")

	switch m.step {
	case createStepTemplate:
		b.WriteString("Select a tech stack template:\n\n")
		for i, t := range scaffold.Templates {
			cursor := "  "
			name := cDimStyle.Render(t.Name)
			desc := cDimStyle.Render(" — " + t.Description)
			if i == m.templateIdx {
				cursor = cSelectedStyle.Render("> ")
				name = cSelectedStyle.Render(t.Name)
			}
			fmt.Fprintf(&b, "%s%s%s\n", cursor, name, desc)
		}
		b.WriteString(cDimStyle.Render("\n↑/↓ navigate • enter select • q quit"))

	case createStepInput:
		tmpl := scaffold.Templates[m.templateIdx]
		fmt.Fprintf(&b, "Template: %s\n\n", cSelectedStyle.Render(tmpl.Name))

		for i := range 4 {
			label := createInputLabels[i]
			if i < m.inputIdx {
				fmt.Fprintf(&b, "  %s: %s\n", cDimStyle.Render(label), cInputStyle.Render(m.inputs[i]))
			} else if i == m.inputIdx {
				val := m.inputs[i]
				placeholder := ""
				if val == "" {
					placeholder = cDimStyle.Render(createInputPlaceholders[i])
				}
				fmt.Fprintf(&b, "  %s: %s%s▏\n", cPromptStyle.Render(label), cInputStyle.Render(val), placeholder)
			} else {
				fmt.Fprintf(&b, "  %s:\n", cDimStyle.Render(label))
			}
		}
		b.WriteString(cDimStyle.Render("\nenter confirm • esc back"))

	case createStepConfirm:
		tmpl := scaffold.Templates[m.templateIdx]
		b.WriteString("Review:\n\n")
		fmt.Fprintf(&b, "  Template:       %s\n", cSelectedStyle.Render(tmpl.Name))
		fmt.Fprintf(&b, "  Project:        %s\n", cInputStyle.Render(m.inputs[0]))
		fmt.Fprintf(&b, "  Registry:       %s\n", cInputStyle.Render(m.inputs[1]))
		fmt.Fprintf(&b, "  Domain:         %s\n", cInputStyle.Render(m.inputs[2]))
		if m.inputs[3] != "" {
			fmt.Fprintf(&b, "  Base registry:  %s\n", cInputStyle.Render(m.inputs[3]))
		}
		b.WriteString(cDimStyle.Render("\nenter/y generate • n restart • esc back"))
	}

	b.WriteString("\n")
	return b.String()
}
