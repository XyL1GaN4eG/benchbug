package metrics

import "sync/atomic"

type Collector struct {
	requests atomic.Int64
	fails    atomic.Int64
}

func NewCollector() *Collector { return &Collector{} }

func (c *Collector) Add(ev Event) {
	c.requests.Add(1)
	if ev.Err != "" || ev.Status >= 400 {
		c.fails.Add(1)
	}
}

func (c *Collector) Summary() Summary {
	return Summary{Requests: c.requests.Load(), Fails: c.fails.Load()}
}
