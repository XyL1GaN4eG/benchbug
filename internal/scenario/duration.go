package scenario

import "time"

type Duration struct{ time.Duration }

func ParseDuration(s string) (Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return Duration{}, err
	}
	return Duration{Duration: d}, nil
}
