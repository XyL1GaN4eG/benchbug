package scenario

type Scenario struct {
	Name     string   `yaml:"name" json:"name"`
	BaseURL  string   `yaml:"base_url" json:"base_url"`
	VUs      int      `yaml:"vus" json:"vus"`
	Duration Duration `yaml:"duration" json:"duration"`
	Tasks    []Task   `yaml:"tasks" json:"tasks"`
}

type Task struct {
	Name   string `yaml:"name" json:"name"`
	Method string `yaml:"method" json:"method"`
	URL    string `yaml:"url" json:"url"`
}
