package metrics

import "time"

type Key struct {
	Group  string
	Step   string
	Method string
	URL    string
}

func (k Key) String() string {
	return k.Group + "|" + k.Step + "|" + k.Method + "|" + k.URL
}

type EventType int

const (
	EventRequest EventType = iota
	EventCheck
	EventDroppedIteration
	EventCompletedIteration
)

type Event struct {
	Type EventType
	Key  Key

	Duration time.Duration
	Status   int
	Err      string
	BytesIn  int
	BytesOut int

	CheckOK bool
}

type Snapshot struct {
	At time.Time

	StageVUs  int
	ActiveVUs int

	Interval   time.Duration
	Reqs       int64
	Fails      int64
	Iterations int64
	Dropped    int64

	P50 time.Duration
	P90 time.Duration
	P95 time.Duration
	P99 time.Duration
}

type Summary struct {
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration

	Requests int64
	Fails    int64
	BytesIn  int64
	BytesOut int64

	ChecksPass        int64
	ChecksFail        int64
	Iterations        int64
	DroppedIterations int64

	P50 time.Duration
	P90 time.Duration
	P95 time.Duration
	P99 time.Duration

	ByKey []KeySummary

	Thresholds []ThresholdResult
}

type KeySummary struct {
	Key      Key
	Requests int64
	Fails    int64
	P95      time.Duration
	P99      time.Duration
}

type ThresholdResult struct {
	Metric string
	Op     string
	Value  string
	Actual string
	OK     bool
}
