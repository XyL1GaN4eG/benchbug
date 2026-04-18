package engine

import "time"

type Options struct {
	File     string
	VUs      int
	Duration time.Duration
	Quiet    bool
}
