package config

type Config struct {
	Env  string `json:"env" required:"true" default:"debug"`
	Port string `json:"port" required:"true" default:"8080"`

	ConfigFolder     string `json:"config_folder" envconfig:"configfolder"`
	IntegrationsFile string `json:"integrations_file" envconfig:"integrations_file"`
}
