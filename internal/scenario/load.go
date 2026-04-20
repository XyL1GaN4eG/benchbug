package scenario

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (*Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s Scenario
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	normalizeScenario(&s)
	if err := Validate(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func normalizeScenario(s *Scenario) {
	if s.Vars == nil {
		s.Vars = map[string]string{}
	}
	if s.Defaults.Headers == nil {
		s.Defaults.Headers = map[string]string{}
	}
	if s.Defaults.Tags == nil {
		s.Defaults.Tags = map[string]string{}
	}
	if s.Arrival != nil && s.Arrival.Per.Duration <= 0 {
		s.Arrival.Per.Duration = time.Second
	}
	for i := range s.Steps {
		if s.Steps[i].Request.Headers == nil {
			s.Steps[i].Request.Headers = map[string]string{}
		}
		if s.Steps[i].Tags == nil {
			s.Steps[i].Tags = map[string]string{}
		}
		if s.Steps[i].Extract.JSONPath == nil {
			s.Steps[i].Extract.JSONPath = map[string]string{}
		}
		if s.Steps[i].Extract.Header == nil {
			s.Steps[i].Extract.Header = map[string]string{}
		}
	}
}

var ErrValidation = errors.New("scenario validation failed")

func Validate(s *Scenario) error {
	var errs []string

	if strings.TrimSpace(s.Name) == "" {
		errs = append(errs, "name is required")
	}
	if len(s.Steps) == 0 {
		errs = append(errs, "steps is required and must be non-empty")
	}
	errs = append(errs, validateLoadModel(s)...)
	errs = append(errs, validateSteps(s.Steps)...)
	errs = append(errs, validateThresholds(s.Threshold)...)

	if len(errs) > 0 {
		return fmt.Errorf("%w:\n- %s", ErrValidation, strings.Join(errs, "\n- "))
	}
	return nil
}

func validateLoadModel(s *Scenario) []string {
	if s.Arrival != nil {
		return validateArrivalRate(s)
	}
	if len(s.Stages) == 0 {
		return validateFlatVUs(s)
	}

	var errs []string
	for i, st := range s.Stages {
		if st.VUs <= 0 {
			errs = append(errs, fmt.Sprintf("stages[%d].vus must be > 0", i))
		}
		if st.Duration.Duration <= 0 {
			errs = append(errs, fmt.Sprintf("stages[%d].duration must be > 0", i))
		}
	}
	return errs
}

func validateArrivalRate(s *Scenario) []string {
	var errs []string
	if s.Arrival.Rate <= 0 {
		errs = append(errs, "arrival_rate.rate must be > 0")
	}
	if s.Arrival.Per.Duration <= 0 {
		errs = append(errs, "arrival_rate.per must be > 0")
	}
	if s.Arrival.Duration.Duration <= 0 {
		errs = append(errs, "arrival_rate.duration must be > 0")
	}
	if s.Arrival.MaxVUs <= 0 {
		errs = append(errs, "arrival_rate.max_vus must be > 0")
	}
	if len(s.Stages) > 0 {
		errs = append(errs, "arrival_rate and stages are mutually exclusive")
	}
	if s.VUs > 0 || s.Duration.Duration > 0 {
		errs = append(errs, "arrival_rate cannot be combined with top-level vus/duration")
	}
	return errs
}

func validateFlatVUs(s *Scenario) []string {
	var errs []string
	if s.VUs <= 0 {
		errs = append(errs, "vus must be > 0 when stages is empty")
	}
	if s.Duration.Duration <= 0 {
		errs = append(errs, "duration must be > 0 when stages is empty")
	}
	return errs
}

func validateSteps(steps []Step) []string {
	var errs []string
	for i, step := range steps {
		if strings.TrimSpace(step.Name) == "" {
			errs = append(errs, fmt.Sprintf("steps[%d].name is required", i))
		}
		m := strings.ToUpper(strings.TrimSpace(step.Request.Method))
		if m == "" {
			errs = append(errs, fmt.Sprintf("steps[%d].request.method is required", i))
		}
		u := strings.TrimSpace(step.Request.URL)
		if u == "" {
			errs = append(errs, fmt.Sprintf("steps[%d].request.url is required", i))
		}
	}
	return errs
}

func validateThresholds(thresholds []Threshold) []string {
	var errs []string
	for i, th := range thresholds {
		if strings.TrimSpace(th.Metric) == "" {
			errs = append(errs, fmt.Sprintf("thresholds[%d].metric is required", i))
		}
		if strings.TrimSpace(th.Op) == "" {
			errs = append(errs, fmt.Sprintf("thresholds[%d].op is required", i))
		}
		if th.Value == nil {
			errs = append(errs, fmt.Sprintf("thresholds[%d].value is required", i))
		}
	}
	return errs
}
