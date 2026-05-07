package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	selfUpdate    bool
	profileImport string
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Manage vela settings and updates",
	RunE:  runConfigure,
}

func init() {
	configureCmd.Flags().BoolVar(&selfUpdate, "self-update", false, "update vela to the latest version")
	configureCmd.Flags().StringVar(&profileImport, "profile", "", "import configuration from a profile file")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	if selfUpdate {
		return runSelfUpdate(cmd)
	}

	if profileImport != "" {
		return runProfileImport(cmd, profileImport)
	}

	p := tea.NewProgram(newConfigureModel())
	_, err := p.Run()
	return err
}

func readProfileYAML(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}

	var raw map[string]string
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse profile YAML: %w", err)
	}

	vals := make(map[string]string)
	for k, v := range raw {
		key := strings.ToUpper(k)
		if !strings.HasPrefix(key, "VELA_") {
			key = "VELA_" + key
		}
		if v != "" {
			vals[key] = v
		}
	}
	return vals, nil
}

func runProfileImport(cmd *cobra.Command, path string) error {
	vals, err := readProfileYAML(path)
	if err != nil {
		return err
	}
	if len(vals) == 0 {
		return fmt.Errorf("no configuration found in %s", path)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\n", path)
	fmt.Fprintf(cmd.OutOrStdout(), "Found %d setting(s):\n\n", len(vals))
	for k, v := range vals {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s = %s\n", k, v)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".vela", ".env")
	projectPath := ""
	if dir, err := project.Find("."); err == nil {
		projectPath = filepath.Join(dir, ".env")
	}

	p := tea.NewProgram(newProfileModel(globalPath, projectPath))
	m, err := p.Run()
	if err != nil {
		return err
	}

	final := m.(profileModel)
	if final.cancelled {
		fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
		return nil
	}

	targetPath := final.targetPath()
	existing := readEnvFileValues(targetPath)
	for k, v := range vals {
		existing[k] = v
	}

	dir := filepath.Dir(targetPath)
	os.MkdirAll(dir, 0755)

	var lines []string
	for k, v := range existing {
		lines = append(lines, k+"="+v)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Imported to %s\n", targetPath)
	return nil
}

// --- Profile import TUI ---

type profileModel struct {
	choices    []string
	paths      []string
	cursor     int
	cancelled  bool
}

func newProfileModel(globalPath, projectPath string) profileModel {
	choices := []string{"Global  (" + globalPath + ")"}
	paths := []string{globalPath}
	if projectPath != "" {
		choices = append(choices, "Project ("+projectPath+")")
		paths = append(paths, projectPath)
	}
	return profileModel{choices: choices, paths: paths}
}

func (m profileModel) targetPath() string {
	return m.paths[m.cursor]
}

func (m profileModel) Init() tea.Cmd { return nil }

func (m profileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m profileModel) View() string {
	var b strings.Builder
	b.WriteString(cfgTitleStyle.Render("Import profile"))
	b.WriteString("\n\n")
	b.WriteString("Import to which scope?\n\n")
	for i, c := range m.choices {
		cursor := "  "
		style := cfgDimStyle
		if i == m.cursor {
			cursor = cfgCursorStyle.Render("> ")
			style = cfgColValue
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, style.Render(c))
	}
	b.WriteString("\n")
	b.WriteString(cfgDimStyle.Render("↑/↓ select • enter confirm • esc cancel"))
	b.WriteString("\n")
	return b.String()
}

// --- TUI Model ---

type configTab int

const (
	configTabInfo configTab = iota
	configTabEdit
)

var configTabs = []string{"Info", "Edit"}

type envScope int

const (
	scopeGlobal  envScope = iota
	scopeProject
)

type envField struct {
	key          string
	label        string
	value        string
	defaultValue string
	scope        envScope
}

type configureModel struct {
	activeTab      configTab
	scrollOffset   int
	width          int
	height         int
	projectState   *state.State
	projectDir     string
	hasProject     bool
	fields         []envField
	editIdx        int
	editing        bool
	editBackup     string
	saved          bool
	savedPath      string
	globalEnvPath  string
	projectEnvPath string
}

var (
	cfgTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	cfgTabActive     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Underline(true)
	cfgTabInactive   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cfgColLabel      = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Width(20)
	cfgColDefault    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(30)
	cfgColValue      = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	cfgColSource     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(10)
	cfgSectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).MarginTop(1)
	cfgDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cfgValueDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	cfgCursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	cfgEditingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	cfgSavedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
)

func newConfigureModel() configureModel {
	m := configureModel{}

	if dir, err := project.Find("."); err == nil {
		m.projectDir = dir
		m.hasProject = true
		m.projectEnvPath = filepath.Join(dir, ".env")
		b := &state.LocalBackend{}
		if s, err := b.Load(dir); err == nil {
			m.projectState = s
		}
	}

	home, _ := os.UserHomeDir()
	m.globalEnvPath = filepath.Join(home, ".vela", ".env")

	m.fields = []envField{
		{key: "VELA_REGISTRY", label: "Registry", defaultValue: "registry.example.com/myteam", scope: scopeGlobal},
		{key: "VELA_DOMAIN", label: "Domain", defaultValue: "apps.example.com", scope: scopeGlobal},
		{key: "VELA_BASE_REGISTRY", label: "Base Registry", defaultValue: "", scope: scopeGlobal},
		{key: "VELA_DB_IMAGE_REGISTRY", label: "DB Image Registry", defaultValue: "", scope: scopeGlobal},
		{key: "VELA_KUBECONFIG", label: "Kubeconfig", defaultValue: "", scope: scopeGlobal},
	}

	if m.hasProject {
		m.fields = append(m.fields,
			envField{key: "VELA_BASE_PATH", label: "Base Path", defaultValue: "", scope: scopeProject},
			envField{key: "VELA_REGISTRY", label: "Registry", defaultValue: "", scope: scopeProject},
			envField{key: "VELA_DOMAIN", label: "Domain", defaultValue: "", scope: scopeProject},
			envField{key: "VELA_BASE_REGISTRY", label: "Base Registry", defaultValue: "", scope: scopeProject},
			envField{key: "VELA_DB_IMAGE_REGISTRY", label: "DB Image Registry", defaultValue: "", scope: scopeProject},
			envField{key: "VELA_KUBECONFIG", label: "Kubeconfig", defaultValue: "", scope: scopeProject},
		)
	}

	globalVals := readEnvFileValues(m.globalEnvPath)
	projectVals := readEnvFileValues(m.projectEnvPath)

	for i := range m.fields {
		switch m.fields[i].scope {
		case scopeGlobal:
			if v, ok := globalVals[m.fields[i].key]; ok {
				m.fields[i].value = v
			}
		case scopeProject:
			if v, ok := projectVals[m.fields[i].key]; ok {
				m.fields[i].value = v
			}
		}
	}

	return m
}

func readEnvFileValues(path string) map[string]string {
	vals := make(map[string]string)
	if path == "" {
		return vals
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return vals
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vals[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return vals
}

func (m configureModel) Init() tea.Cmd {
	return nil
}

func (m configureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case envSavedMsg:
		m.saved = true
		m.savedPath = msg.path
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.editing {
			return m.updateEditing(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.activeTab == configTabEdit {
				m.activeTab = configTabInfo
				m.saved = false
			} else {
				return m, tea.Quit
			}
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % configTab(len(configTabs))
			m.scrollOffset = 0
			m.saved = false
		case "shift+tab", "left", "h":
			if m.activeTab == 0 {
				m.activeTab = configTab(len(configTabs) - 1)
			} else {
				m.activeTab--
			}
			m.scrollOffset = 0
			m.saved = false
		case "down", "j":
			if m.activeTab == configTabEdit {
				if m.editIdx < len(m.fields)-1 {
					m.editIdx++
				}
			} else {
				m.scrollOffset++
			}
		case "up", "k":
			if m.activeTab == configTabEdit {
				if m.editIdx > 0 {
					m.editIdx--
				}
			} else if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "enter":
			if m.activeTab == configTabEdit {
				m.editing = true
				m.editBackup = m.fields[m.editIdx].value
				m.saved = false
			}
		}
	}
	return m, nil
}

func (m configureModel) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.editing = false
		return m, m.saveFieldScope(m.fields[m.editIdx].scope)
	case tea.KeyEscape:
		m.fields[m.editIdx].value = m.editBackup
		m.editing = false
	case tea.KeyBackspace:
		if len(m.fields[m.editIdx].value) > 0 {
			m.fields[m.editIdx].value = m.fields[m.editIdx].value[:len(m.fields[m.editIdx].value)-1]
		}
	case tea.KeyCtrlU:
		m.fields[m.editIdx].value = ""
		m.editing = false
		return m, m.saveFieldScope(m.fields[m.editIdx].scope)
	case tea.KeyRunes:
		m.fields[m.editIdx].value += string(msg.Runes)
	case tea.KeySpace:
		m.fields[m.editIdx].value += " "
	}
	return m, nil
}

type envSavedMsg struct{ path string }

func (m configureModel) saveFieldScope(scope envScope) tea.Cmd {
	return func() tea.Msg {
		var targetPath string
		if scope == scopeProject && m.projectEnvPath != "" {
			targetPath = m.projectEnvPath
		} else {
			targetPath = m.globalEnvPath
		}

		existing := readEnvFileValues(targetPath)

		for _, f := range m.fields {
			if f.scope != scope {
				continue
			}
			if f.value != "" {
				existing[f.key] = f.value
			} else {
				delete(existing, f.key)
			}
		}

		dir := filepath.Dir(targetPath)
		os.MkdirAll(dir, 0755)

		var lines []string
		for k, v := range existing {
			lines = append(lines, k+"="+v)
		}
		content := strings.Join(lines, "\n") + "\n"
		if len(existing) == 0 {
			content = ""
		}
		os.WriteFile(targetPath, []byte(content), 0644)
		return envSavedMsg{path: targetPath}
	}
}

func (m configureModel) View() string {
	var b strings.Builder

	b.WriteString(cfgTitleStyle.Render("vela configure"))
	b.WriteString("\n\n")

	var tabs []string
	for i, t := range configTabs {
		if configTab(i) == m.activeTab {
			tabs = append(tabs, cfgTabActive.Render(t))
		} else {
			tabs = append(tabs, cfgTabInactive.Render(t))
		}
	}
	b.WriteString(strings.Join(tabs, "    "))
	b.WriteString("\n\n")

	switch m.activeTab {
	case configTabInfo:
		b.WriteString(m.viewInfo())
		b.WriteString("\n")
		b.WriteString(cfgDimStyle.Render("tab/→ switch • ↑/↓ scroll • esc exit"))
	case configTabEdit:
		b.WriteString(m.viewEdit())
		b.WriteString("\n")
		if m.editing {
			b.WriteString(cfgDimStyle.Render("type to edit • enter confirm • esc cancel • ctrl+u clear"))
		} else {
			hint := "↑/↓ select • enter edit • esc back"
			if m.saved {
				hint = cfgSavedStyle.Render("✓ Saved to "+m.savedPath) + "  " + cfgDimStyle.Render("esc back")
			}
			b.WriteString(hint)
		}
	}

	b.WriteString("\n")
	return b.String()
}

func (m configureModel) infoRow(label, def, cur, source string) string {
	return "  " + cfgColLabel.Render(label) + cfgColDefault.Render(def) + cfgColValue.Render(cur) + "  " + cfgColSource.Render(source) + "\n"
}

func (m configureModel) kvRow(label, value string) string {
	return "  " + cfgColLabel.Render(label) + cfgColValue.Render(value) + "\n"
}

func resolveSource(key, globalPath, projectPath string) string {
	if os.Getenv(key) != "" {
		pVals := readEnvFileValues(projectPath)
		if _, ok := pVals[key]; ok {
			return "project"
		}
		gVals := readEnvFileValues(globalPath)
		if _, ok := gVals[key]; ok {
			return "global"
		}
		return "env"
	}
	return ""
}

func (m configureModel) viewInfo() string {
	var b strings.Builder

	b.WriteString(cfgSectionStyle.Render("Configuration"))
	b.WriteString("\n")
	headerLabel := cfgColLabel.Underline(true)
	headerDefault := cfgColDefault.Underline(true)
	headerValue := cfgColValue.Underline(true)
	headerSource := cfgColSource.Underline(true)
	b.WriteString("  " + headerLabel.Render("Setting") +
		headerDefault.Render("Default") +
		headerValue.Render("Current Value") + "  " +
		headerSource.Render("Source") + "\n")

	type row struct {
		key, label, def string
	}
	rows := []row{
		{"VELA_REGISTRY", "Registry", "registry.example.com/myteam"},
		{"VELA_DOMAIN", "Domain", "apps.example.com"},
		{"VELA_BASE_REGISTRY", "Base Registry", "(none)"},
		{"VELA_DB_IMAGE_REGISTRY", "DB Image Registry", "(none)"},
		{"VELA_KUBECONFIG", "Kubeconfig", "(none)"},
		{"VELA_BASE_PATH", "Base Path", "(none)"},
	}

	for _, r := range rows {
		cur := os.Getenv(r.key)
		source := resolveSource(r.key, m.globalEnvPath, m.projectEnvPath)
		if cur == "" {
			cur = "(not set)"
			source = ""
		}
		b.WriteString(m.infoRow(r.label, r.def, cur, source))
	}

	b.WriteString("\n")
	b.WriteString(cfgSectionStyle.Render("System"))
	b.WriteString("\n")
	b.WriteString(m.kvRow("Version", Version))
	b.WriteString(m.kvRow("Platform", runtime.GOOS+"/"+runtime.GOARCH))
	b.WriteString(m.kvRow("Global config", m.globalEnvPath))
	if m.hasProject {
		b.WriteString(m.kvRow("Project config", m.projectEnvPath))
	}

	if m.hasProject {
		b.WriteString("\n")
		b.WriteString(cfgSectionStyle.Render("Project"))
		b.WriteString("\n")
		b.WriteString(m.kvRow("Directory", m.projectDir))
		if m.projectState != nil {
			b.WriteString(m.kvRow("Name", m.projectState.Name))
			b.WriteString(m.kvRow("Namespace", m.projectState.Namespace))
			b.WriteString(m.kvRow("Status", m.projectState.Status))
			if m.projectState.LastDeployed != "" {
				b.WriteString(m.kvRow("Last Deployed", m.projectState.LastDeployed))
			}
			if m.projectState.Revision > 0 {
				b.WriteString(m.kvRow("Revision", fmt.Sprintf("%d", m.projectState.Revision)))
			}
			if len(m.projectState.Services) > 0 {
				b.WriteString("\n")
				b.WriteString(cfgSectionStyle.Render("Services"))
				b.WriteString("\n")
				for name, svc := range m.projectState.Services {
					b.WriteString(m.kvRow(name, svc.Image))
					if svc.IngressPath != "" {
						b.WriteString(m.kvRow("  ingress", svc.IngressPath))
					}
				}
			}
		}
	} else {
		b.WriteString("\n")
		b.WriteString(cfgDimStyle.Render("  (not inside a vela project)"))
		b.WriteString("\n")
	}

	return b.String()
}

func (m configureModel) viewEdit() string {
	var b strings.Builder

	// Group by scope
	var lastScope envScope = -1
	for i, f := range m.fields {
		if f.scope != lastScope {
			lastScope = f.scope
			if f.scope == scopeGlobal {
				b.WriteString(cfgSectionStyle.Render("Global"))
				b.WriteString(cfgDimStyle.Render("  → " + m.globalEnvPath))
				b.WriteString("\n")
			} else {
				target := m.projectEnvPath
				if target == "" {
					target = "(no project)"
				}
				b.WriteString("\n")
				b.WriteString(cfgSectionStyle.Render("Project"))
				b.WriteString(cfgDimStyle.Render("  → " + target))
				b.WriteString("\n")
			}
		}

		cursor := "  "
		if i == m.editIdx {
			cursor = cfgCursorStyle.Render("> ")
		}

		label := cfgColLabel.Render(f.label)

		var val string
		if i == m.editIdx && m.editing {
			val = cfgEditingStyle.Render(f.value) + cfgCursorStyle.Render("▏")
		} else if f.value != "" {
			val = cfgColValue.Render(f.value)
		} else {
			val = cfgValueDimStyle.Render("(empty)")
		}

		fmt.Fprintf(&b, "%s%s %s\n", cursor, label, val)

		if i == m.editIdx && !m.editing {
			hint := ""
			if f.defaultValue != "" {
				hint = cfgDimStyle.Render("    default: " + f.defaultValue)
			} else if f.key == "VELA_BASE_PATH" {
				hint = cfgDimStyle.Render("    e.g. /sandbox/myapp (project-specific path prefix)")
			} else if f.key == "VELA_KUBECONFIG" {
				hint = cfgDimStyle.Render("    e.g. ~/.kube/k3s-config (path to kubeconfig file)")
			}
			if hint != "" {
				b.WriteString(hint + "\n")
			}
		}
	}

	return b.String()
}


type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

const repoAPI = "https://api.github.com/repos/khala-matrix/vela/releases/latest"

func runSelfUpdate(cmd *cobra.Command) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", Version)
	fmt.Fprintln(cmd.OutOrStdout(), "Checking for updates...")

	resp, err := http.Get(repoAPI)
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")
	if latest == current {
		fmt.Fprintln(cmd.OutOrStdout(), "Already up to date.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "New version available: %s → %s\n", Version, release.TagName)

	assetName := fmt.Sprintf("vela_%s_%s", runtime.GOOS, runtime.GOARCH)
	checksumName := "checksums.txt"

	var assetURL, checksumURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
		}
		if a.Name == checksumName {
			checksumURL = a.BrowserDownloadURL
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", assetName)
	binData, err := downloadFile(assetURL)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}

	if checksumURL != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "Verifying checksum...")
		checksumData, err := downloadFile(checksumURL)
		if err != nil {
			return fmt.Errorf("download checksums: %w", err)
		}
		if err := verifyFileChecksum(binData, checksumData, assetName); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable path: %w", err)
	}

	tmpFile := self + ".new"
	if err := os.WriteFile(tmpFile, binData, 0755); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}

	if err := os.Rename(tmpFile, self); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s successfully.\n", release.TagName)
	return nil
}

func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func verifyFileChecksum(data []byte, checksumFile []byte, name string) error {
	hash := sha256.Sum256(data)
	got := hex.EncodeToString(hash[:])

	for _, line := range strings.Split(string(checksumFile), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == name {
			if parts[0] == got {
				return nil
			}
			return fmt.Errorf("expected %s, got %s", parts[0], got)
		}
	}
	return fmt.Errorf("no checksum found for %s", name)
}
