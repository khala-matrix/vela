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
}

type ServiceState struct {
	Image       string `yaml:"image"`
	IngressPath string `yaml:"ingress_path,omitempty"`
}

type Backend interface {
	Load(projectDir string) (*State, error)
	Save(projectDir string, state *State) error
}
