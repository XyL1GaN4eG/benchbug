package scenario

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (*Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil, fmt.Errorf("scenario file is empty")
	}
	var raw struct {
		Name     string `yaml:"name"`
		BaseURL  string `yaml:"base_url"`
		VUs      int    `yaml:"vus"`
		Duration string `yaml:"duration"`
		Tasks    []Task `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	d, err := ParseDuration(raw.Duration)
	if err != nil {
		return nil, fmt.Errorf("bad duration: %w", err)
	}
	sc := &Scenario{Name: raw.Name, BaseURL: raw.BaseURL, VUs: raw.VUs, Duration: d, Steps: raw.Steps}
	if err := Validate(sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func Validate(sc *Scenario) error {
	var errs []string
	if strings.TrimSpace(sc.Name) == "" {
		errs = append(errs, "name is required")
	}
	if sc.VUs <= 0 {
		errs = append(errs, "vus must be > 0")
	}
	if sc.Duration.Duration <= 0 {
		errs = append(errs, "duration must be > 0")
	}
	if len(sc.Steps) == 0 {
		errs = append(errs, "steps must be non-empty")
	}
	if len(errs) > 0 {
		return fmt.Errorf("scenario validation failed:\n- %s", strings.Join(errs, "\n- "))
	}
	return nil
}
