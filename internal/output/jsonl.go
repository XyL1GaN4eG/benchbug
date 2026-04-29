package output

import (
	"bufio"
	"encoding/json"
	"io"

	"benchbug/internal/metrics"
)

type JSONL struct {
	bw *bufio.Writer
	w  io.WriteCloser
}

func NewJSONL(w io.WriteCloser) *JSONL {
	return &JSONL{
		bw: bufio.NewWriterSize(w, 256*1024),
		w:  w,
	}
}

func (j *JSONL) OnSnapshot(s metrics.Snapshot) {
	_ = json.NewEncoder(j.bw).Encode(map[string]any{
		"type":       "snapshot",
		"at":         s.At,
		"stage_vus":  s.StageVUs,
		"active_vus": s.ActiveVUs,
		"interval":   s.Interval.String(),
		"reqs":       s.Reqs,
		"fails":      s.Fails,
		"iterations": s.Iterations,
		"dropped":    s.Dropped,
		"p50":        s.P50.String(),
		"p90":        s.P90.String(),
		"p95":        s.P95.String(),
		"p99":        s.P99.String(),
	})
	_ = j.bw.Flush()
}

func (j *JSONL) OnSummary(sum metrics.Summary) {
	_ = json.NewEncoder(j.bw).Encode(map[string]any{
		"type":        "summary",
		"started_at":  sum.StartedAt,
		"ended_at":    sum.EndedAt,
		"duration":    sum.Duration.String(),
		"requests":    sum.Requests,
		"fails":       sum.Fails,
		"bytes_in":    sum.BytesIn,
		"bytes_out":   sum.BytesOut,
		"checks_pass": sum.ChecksPass,
		"checks_fail": sum.ChecksFail,
		"iterations":  sum.Iterations,
		"dropped":     sum.DroppedIterations,
		"p50":         sum.P50.String(),
		"p90":         sum.P90.String(),
		"p95":         sum.P95.String(),
		"p99":         sum.P99.String(),
		"thresholds":  sum.Thresholds,
		"top":         sum.ByKey,
	})
	_ = j.bw.Flush()
}

func (j *JSONL) Close() error {
	_ = j.bw.Flush()
	return j.w.Close()
}
