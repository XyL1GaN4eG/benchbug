package output

import (
	"fmt"
	"io"
	"time"

	"benchbug/internal/metrics"
)

type Console struct {
	w io.Writer
}

func NewConsole(w io.Writer) *Console {
	return &Console{w: w}
}

func (c *Console) OnSnapshot(s metrics.Snapshot) {
	sec := s.Interval.Seconds()
	rps := float64(s.Reqs)
	if sec > 0 {
		rps = float64(s.Reqs) / sec
	}
	failRate := 0.0
	if s.Reqs > 0 {
		failRate = float64(s.Fails) / float64(s.Reqs)
	}
	_, _ = fmt.Fprintf(c.w, "%s target_vus=%d active_vus=%d rps=%.1f iter=%d fail=%.2f%% dropped=%d p95=%s p99=%s\n",
		s.At.Format(time.RFC3339),
		s.StageVUs,
		s.ActiveVUs,
		rps,
		s.Iterations,
		failRate*100.0,
		s.Dropped,
		s.P95,
		s.P99,
	)
}

func (c *Console) OnSummary(sum metrics.Summary) {
	failRate := 0.0
	if sum.Requests > 0 {
		failRate = float64(sum.Fails) / float64(sum.Requests)
	}
	checkRate := 1.0
	totalChecks := sum.ChecksPass + sum.ChecksFail
	if totalChecks > 0 {
		checkRate = float64(sum.ChecksPass) / float64(totalChecks)
	}

	_, _ = fmt.Fprintf(c.w, "\nSUMMARY duration=%s iterations=%d requests=%d fail=%.2f%% dropped=%d bytes_in=%d bytes_out=%d checks=%.2f%% p50=%s p95=%s p99=%s\n",
		sum.Duration,
		sum.Iterations,
		sum.Requests,
		failRate*100.0,
		sum.DroppedIterations,
		sum.BytesIn,
		sum.BytesOut,
		checkRate*100.0,
		sum.P50, sum.P95, sum.P99,
	)
	if len(sum.Thresholds) > 0 {
		_, _ = fmt.Fprintln(c.w, "THRESHOLDS")
		for _, th := range sum.Thresholds {
			status := "FAIL"
			if th.OK {
				status = "OK"
			}
			_, _ = fmt.Fprintf(c.w, "  %s: %s %s %s (actual %s)\n", status, th.Metric, th.Op, th.Value, th.Actual)
		}
	}
	if len(sum.ByKey) > 0 {
		_, _ = fmt.Fprintln(c.w, "TOP")
		for _, row := range sum.ByKey {
			_, _ = fmt.Fprintf(c.w, "  %s req=%d fail=%d p95=%s p99=%s\n",
				row.Key.String(), row.Requests, row.Fails, row.P95, row.P99)
		}
	}
}
