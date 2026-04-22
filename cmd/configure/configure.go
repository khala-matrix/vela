package configure

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mars/vela/pkg/scaffold"
	"github.com/spf13/cobra"
)

var ConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Generate a tech-stack.yaml from a template",
	RunE:  runConfigure,
}

func runConfigure(cmd *cobra.Command, args []string) error {
	p := tea.NewProgram(newModel())
	m, err := p.Run()
	if err != nil {
		return err
	}

	final := m.(model)
	if final.cancelled {
		fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
		return nil
	}

	tmpl := scaffold.Templates[final.templateIdx]
	params := scaffold.Params{
		Name:     final.inputs[0],
		Registry: final.inputs[1],
		Domain:   final.inputs[2],
	}

	outDir := params.Name
	if err := scaffold.RenderSkeleton(tmpl.ID, params, outDir); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Generated project in %s/ using %q template.\n", outDir, tmpl.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Run 'vela app create %s -f %s/tech-stack.yaml' to create the app.\n", params.Name, outDir)
	return nil
}

type step int

const (
	stepTemplate step = iota
	stepInput
	stepConfirm
)

type model struct {
	step        step
	templateIdx int
	inputIdx    int
	inputs      [3]string
	cancelled   bool
	width       int
}

var inputLabels = [3]string{"Project name", "Image registry", "Ingress domain"}
var inputPlaceholders = [3]string{"my-app", "registry.example.com/namespace", "example.com"}

func newModel() model {
	return model{}
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	inputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
)

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.step == stepTemplate {
				m.cancelled = true
				return m, tea.Quit
			}
			if msg.String() == "ctrl+c" {
				m.cancelled = true
				return m, tea.Quit
			}
		case "esc":
			if m.step == stepInput && m.inputIdx > 0 {
				m.inputIdx--
				return m, nil
			}
			if m.step == stepInput && m.inputIdx == 0 {
				m.step = stepTemplate
				return m, nil
			}
			if m.step == stepConfirm {
				m.step = stepInput
				m.inputIdx = 2
				return m, nil
			}
		}
	}

	switch m.step {
	case stepTemplate:
		return m.updateTemplate(msg)
	case stepInput:
		return m.updateInput(msg)
	case stepConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

func (m model) updateTemplate(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.step = stepInput
		}
	}
	return m, nil
}

func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter":
			if m.inputs[m.inputIdx] == "" {
				m.inputs[m.inputIdx] = inputPlaceholders[m.inputIdx]
			}
			if m.inputIdx < 2 {
				m.inputIdx++
			} else {
				m.step = stepConfirm
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

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter", "y":
			return m, tea.Quit
		case "n":
			m.step = stepInput
			m.inputIdx = 0
			m.inputs = [3]string{}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("⛵ vela configure"))
	b.WriteString("\n\n")

	switch m.step {
	case stepTemplate:
		b.WriteString("Select a tech stack template:\n\n")
		for i, t := range scaffold.Templates {
			cursor := "  "
			name := dimStyle.Render(t.Name)
			desc := dimStyle.Render(" — " + t.Description)
			if i == m.templateIdx {
				cursor = selectedStyle.Render("> ")
				name = selectedStyle.Render(t.Name)
				desc = dimStyle.Render(" — " + t.Description)
			}
			b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, name, desc))
		}
		b.WriteString(dimStyle.Render("\n↑/↓ navigate • enter select • q quit"))

	case stepInput:
		tmpl := scaffold.Templates[m.templateIdx]
		b.WriteString(fmt.Sprintf("Template: %s\n\n", selectedStyle.Render(tmpl.Name)))

		for i := 0; i < 3; i++ {
			label := inputLabels[i]
			if i < m.inputIdx {
				b.WriteString(fmt.Sprintf("  %s: %s\n", dimStyle.Render(label), inputStyle.Render(m.inputs[i])))
			} else if i == m.inputIdx {
				val := m.inputs[i]
				placeholder := ""
				if val == "" {
					placeholder = dimStyle.Render(inputPlaceholders[i])
				}
				b.WriteString(fmt.Sprintf("  %s: %s%s▏\n", promptStyle.Render(label), inputStyle.Render(val), placeholder))
			} else {
				b.WriteString(fmt.Sprintf("  %s:\n", dimStyle.Render(label)))
			}
		}
		b.WriteString(dimStyle.Render("\nenter confirm • esc back"))

	case stepConfirm:
		tmpl := scaffold.Templates[m.templateIdx]
		b.WriteString("Review:\n\n")
		b.WriteString(fmt.Sprintf("  Template: %s\n", selectedStyle.Render(tmpl.Name)))
		b.WriteString(fmt.Sprintf("  Project:  %s\n", inputStyle.Render(m.inputs[0])))
		b.WriteString(fmt.Sprintf("  Registry: %s\n", inputStyle.Render(m.inputs[1])))
		b.WriteString(fmt.Sprintf("  Domain:   %s\n", inputStyle.Render(m.inputs[2])))
		b.WriteString(dimStyle.Render("\nenter/y generate • n restart • esc back"))
	}

	b.WriteString("\n")
	return b.String()
}

func init() {
	// Suppress bubbletea's alt screen
	os.Setenv("TERM_PROGRAM", "")
}
