package engine

import "time"

type Options struct {
	File     string
	VUs      int
	Duration time.Duration
	Rate     float64
	MaxVUs   int
	Seed     int64
	JSON     bool
	Timeout  time.Duration
	Insecure bool
	MaxBody  int64
	Quiet    bool
}
