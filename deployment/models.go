package deployment

type DeploymentRequest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas int `yaml:"replicas"`
	} `yaml:"spec"`
	Container struct {
		Name          string `yaml:"name"`
		Image         string `yaml:"image"`
		Binding       string `yaml:"binding"`
		ContainerPort string `yaml:"containerPort"`
		HostPort      string `yaml:"hostPort"`
		EnvVars       []struct {
			Name  string `yaml:"name"`
			Value string `yaml:"value"`
		} `yaml:"envVars"`
		Secrets []Secret `yaml:"secrets"`
	} `yaml:"container"`
}

type Secret struct {
	SecretPath  string `json:"secretPath"`
	SecretKey   string `json:"secretKey"`
	SecretValue string `json:"secretValue,omitempty"`
}
