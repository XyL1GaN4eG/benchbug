package metrics

import (
	"sort"
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

type Collector struct {
	events chan Event

	startedAt time.Time
	endedAt   atomic.Value

	all   *stats
	byKey map[string]*stats

	stageVUs   atomic.Int64
	activeVUs  atomic.Int64
	iterations atomic.Int64
	dropped    atomic.Int64
}

type stats struct {
	key Key

	requests int64
	fails    int64
	bytesIn  int64
	bytesOut int64

	checkPass int64
	checkFail int64

	h *hdrhistogram.Histogram
}

func newStats(key Key) *stats {
	return &stats{
		key: key,
		h:   hdrhistogram.New(1, int64((10*time.Minute)/time.Microsecond), 3),
	}
}

type Options struct {
	EventsBuffer int
}

func NewCollector(opts Options) *Collector {
	if opts.EventsBuffer <= 0 {
		opts.EventsBuffer = 8192
	}
	c := &Collector{
		events:    make(chan Event, opts.EventsBuffer),
		startedAt: time.Now(),
		all:       newStats(Key{Group: "__all__"}),
		byKey:     map[string]*stats{},
	}
	return c
}

func (c *Collector) Events() chan<- Event { return c.events }

func (c *Collector) SetStageVUs(v int)  { c.stageVUs.Store(int64(v)) }
func (c *Collector) SetActiveVUs(v int) { c.activeVUs.Store(int64(v)) }

func (c *Collector) Run(stop <-chan struct{}, snapshots chan<- Snapshot) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var (
		lastAt         = time.Now()
		lastReqs       int64
		lastFail       int64
		lastIterations int64
		lastDropped    int64
	)

	for {
		select {
		case <-stop:
			for {
				select {
				case ev := <-c.events:
					c.apply(ev)
				default:
					c.endedAt.Store(time.Now())
					return
				}
			}
		case ev := <-c.events:
			c.apply(ev)
		case now := <-ticker.C:
			for {
				select {
				case ev := <-c.events:
					c.apply(ev)
				default:
					goto drained
				}
			}
		drained:
			reqs := atomic.LoadInt64(&c.all.requests)
			fails := atomic.LoadInt64(&c.all.fails)
			iterations := c.iterations.Load()
			dropped := c.dropped.Load()

			interval := now.Sub(lastAt)
			dReqs := reqs - lastReqs
			dFails := fails - lastFail
			dIterations := iterations - lastIterations
			dDropped := dropped - lastDropped

			h := c.all.h
			p50 := time.Duration(h.ValueAtQuantile(50.0)) * time.Microsecond
			p90 := time.Duration(h.ValueAtQuantile(90.0)) * time.Microsecond
			p95 := time.Duration(h.ValueAtQuantile(95.0)) * time.Microsecond
			p99 := time.Duration(h.ValueAtQuantile(99.0)) * time.Microsecond

			snapshots <- Snapshot{
				At:         now,
				StageVUs:   int(c.stageVUs.Load()),
				ActiveVUs:  int(c.activeVUs.Load()),
				Interval:   interval,
				Reqs:       dReqs,
				Fails:      dFails,
				Iterations: dIterations,
				Dropped:    dDropped,
				P50:        p50,
				P90:        p90,
				P95:        p95,
				P99:        p99,
			}

			lastAt = now
			lastReqs = reqs
			lastFail = fails
			lastIterations = iterations
			lastDropped = dropped
		}
	}
}

func (c *Collector) apply(ev Event) {
	switch ev.Type {
	case EventRequest:
		c.addRequest(c.all, ev)
		k := ev.Key.String()
		st, ok := c.byKey[k]
		if !ok {
			st = newStats(ev.Key)
			c.byKey[k] = st
		}
		c.addRequest(st, ev)
	case EventCheck:
		if ev.CheckOK {
			atomic.AddInt64(&c.all.checkPass, 1)
		} else {
			atomic.AddInt64(&c.all.checkFail, 1)
		}
		k := ev.Key.String()
		st, ok := c.byKey[k]
		if !ok {
			st = newStats(ev.Key)
			c.byKey[k] = st
		}
		if ev.CheckOK {
			atomic.AddInt64(&st.checkPass, 1)
		} else {
			atomic.AddInt64(&st.checkFail, 1)
		}
	case EventDroppedIteration:
		c.dropped.Add(1)
	case EventCompletedIteration:
		c.iterations.Add(1)
	}
}

func (c *Collector) addRequest(st *stats, ev Event) {
	atomic.AddInt64(&st.requests, 1)
	atomic.AddInt64(&st.bytesIn, int64(ev.BytesIn))
	atomic.AddInt64(&st.bytesOut, int64(ev.BytesOut))
	if ev.Err != "" || ev.Status >= 400 {
		atomic.AddInt64(&st.fails, 1)
	}
	us := ev.Duration / time.Microsecond
	if us < 1 {
		us = 1
	}
	_ = st.h.RecordValue(int64(us))
}

func (c *Collector) SummaryTopN(topN int) Summary {
	end, _ := c.endedAt.Load().(time.Time)
	if end.IsZero() {
		end = time.Now()
	}
	all := c.all
	sum := Summary{
		StartedAt:         c.startedAt,
		EndedAt:           end,
		Duration:          end.Sub(c.startedAt),
		Requests:          atomic.LoadInt64(&all.requests),
		Fails:             atomic.LoadInt64(&all.fails),
		BytesIn:           atomic.LoadInt64(&all.bytesIn),
		BytesOut:          atomic.LoadInt64(&all.bytesOut),
		ChecksPass:        atomic.LoadInt64(&all.checkPass),
		ChecksFail:        atomic.LoadInt64(&all.checkFail),
		Iterations:        c.iterations.Load(),
		DroppedIterations: c.dropped.Load(),
		P50:               time.Duration(all.h.ValueAtQuantile(50.0)) * time.Microsecond,
		P90:               time.Duration(all.h.ValueAtQuantile(90.0)) * time.Microsecond,
		P95:               time.Duration(all.h.ValueAtQuantile(95.0)) * time.Microsecond,
		P99:               time.Duration(all.h.ValueAtQuantile(99.0)) * time.Microsecond,
	}

	if topN <= 0 {
		topN = 20
	}
	var rows []KeySummary
	rows = make([]KeySummary, 0, len(c.byKey))
	for _, st := range c.byKey {
		reqs := atomic.LoadInt64(&st.requests)
		if reqs == 0 {
			continue
		}
		rows = append(rows, KeySummary{
			Key:      st.key,
			Requests: reqs,
			Fails:    atomic.LoadInt64(&st.fails),
			P95:      time.Duration(st.h.ValueAtQuantile(95.0)) * time.Microsecond,
			P99:      time.Duration(st.h.ValueAtQuantile(99.0)) * time.Microsecond,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Requests > rows[j].Requests })
	if len(rows) > topN {
		rows = rows[:topN]
	}
	sum.ByKey = rows
	return sum
}
