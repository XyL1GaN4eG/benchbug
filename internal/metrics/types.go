package metrics

import "time"

type Key struct{ Group, Step, Method, URL string }

func (k Key) String() string { return k.Group + "|" + k.Step + "|" + k.Method + "|" + k.URL }

type Event struct {
	Key      Key
	Duration time.Duration
	Status   int
	Err      string
}

type Summary struct {
	Requests int64
	Fails    int64
	P95      time.Duration
}
