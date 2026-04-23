package state

const (
	StatusCreated  = "created"
	StatusDeployed = "deployed"
	StatusFailed   = "failed"
	StatusDeleted  = "deleted"
)

type State struct {
	Name         string                  `yaml:"name" json:"name"`
	Namespace    string                  `yaml:"namespace" json:"namespace"`
	Cluster      string                  `yaml:"cluster,omitempty" json:"cluster,omitempty"`
	LastDeployed string                  `yaml:"last_deployed,omitempty" json:"last_deployed,omitempty"`
	Revision     int                     `yaml:"revision,omitempty" json:"revision,omitempty"`
	Status       string                  `yaml:"status" json:"status"`
	Services     map[string]ServiceState `yaml:"services,omitempty" json:"services,omitempty"`
	Credentials  map[string]*Credential  `yaml:"credentials,omitempty" json:"credentials,omitempty"`
}

type ServiceState struct {
	Image       string `yaml:"image" json:"image"`
	IngressPath string `yaml:"ingress_path,omitempty" json:"ingress_path,omitempty"`
}

type Credential struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Database string `yaml:"database" json:"database"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
}

type Backend interface {
	Load(projectDir string) (*State, error)
	Save(projectDir string, state *State) error
}
