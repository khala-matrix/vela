package state

const (
	StatusCreated  = "created"
	StatusDeployed = "deployed"
	StatusFailed   = "failed"
	StatusDeleted  = "deleted"
)

type State struct {
	Name         string                  `yaml:"name"`
	Namespace    string                  `yaml:"namespace"`
	Cluster      string                  `yaml:"cluster,omitempty"`
	LastDeployed string                  `yaml:"last_deployed,omitempty"`
	Revision     int                     `yaml:"revision,omitempty"`
	Status       string                  `yaml:"status"`
	Services     map[string]ServiceState `yaml:"services,omitempty"`
	Credentials  map[string]*Credential  `yaml:"credentials,omitempty"`
}

type ServiceState struct {
	Image       string `yaml:"image"`
	IngressPath string `yaml:"ingress_path,omitempty"`
}

type Credential struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type Backend interface {
	Load(projectDir string) (*State, error)
	Save(projectDir string, state *State) error
}
