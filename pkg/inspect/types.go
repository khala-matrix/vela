package inspect

type StackType string

const (
	StackNextjsFastapi   StackType = "nextjs-fastapi"
	StackNextjsFastapiPg StackType = "nextjs-fastapi-pg"
	StackStaticSite      StackType = "static-site"
	StackUnsupported     StackType = "unsupported"
)

var SupportedStacks = []StackType{
	StackNextjsFastapi,
	StackNextjsFastapiPg,
	StackStaticSite,
}

type ScanResult struct {
	ProjectPath      string
	Tree             string
	HasPackageJSON   bool
	HasRequirements  bool
	HasGoMod         bool
	HasDockerfile    bool
	HasDockerCompose bool
	HasNextConfig    bool
	HasNginxConf     bool
	PackageJSONDeps  map[string]string
	PythonDeps       []string
	EnvVars          []string
	DetectedStack    StackType
	DetectedReason   string
}

type AnalysisResult struct {
	Summary       string       `json:"summary"`
	TechStackYAML string       `json:"tech_stack_yaml"`
	BuildSH       string       `json:"build_sh"`
	FileChanges   []FileChange `json:"file_changes"`
	Issues        []Issue      `json:"issues"`
}

type FileChange struct {
	File             string `json:"file"`
	Action           string `json:"action"`
	Description      string `json:"description"`
	CurrentSnippet   string `json:"current_snippet,omitempty"`
	SuggestedSnippet string `json:"suggested_snippet"`
}

type Issue struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type PlanResult struct {
	Plan string `json:"plan"`
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
