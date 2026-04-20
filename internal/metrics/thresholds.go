package metrics

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ThresholdSpec struct {
	Metric string
	Op     string
	Value  string
}

func EvaluateThresholds(specs []ThresholdSpec, sum Summary) []ThresholdResult {
	var out []ThresholdResult
	for _, s := range specs {
		tr := ThresholdResult{
			Metric: s.Metric,
			Op:     s.Op,
			Value:  s.Value,
		}
		actual, ok, err := metricValue(s.Metric, sum)
		if err != nil {
			tr.Actual = "error: " + err.Error()
			tr.OK = false
			out = append(out, tr)
			continue
		}
		if !ok {
			tr.Actual = "unknown metric"
			tr.OK = false
			out = append(out, tr)
			continue
		}

		switch actual.Kind {
		case kindFloat:
			want, err := parseFloat(s.Value)
			if err != nil {
				tr.Actual = fmt.Sprintf("%g", actual.F)
				tr.OK = false
				out = append(out, tr)
				continue
			}
			tr.Actual = fmt.Sprintf("%g", actual.F)
			tr.OK = compareFloat(actual.F, want, s.Op)
		case kindDuration:
			want, err := time.ParseDuration(strings.TrimSpace(s.Value))
			if err != nil {
				tr.Actual = actual.D.String()
				tr.OK = false
				out = append(out, tr)
				continue
			}
			tr.Actual = actual.D.String()
			tr.OK = compareDuration(actual.D, want, s.Op)
		}

		out = append(out, tr)
	}
	return out
}

type valueKind int

const (
	kindFloat valueKind = iota
	kindDuration
)

type metricActual struct {
	Kind valueKind
	F    float64
	D    time.Duration
}

func metricValue(metric string, sum Summary) (metricActual, bool, error) {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "http_req_failed_rate":
		if sum.Requests == 0 {
			return metricActual{Kind: kindFloat, F: 0}, true, nil
		}
		return metricActual{Kind: kindFloat, F: float64(sum.Fails) / float64(sum.Requests)}, true, nil
	case "checks_pass_rate":
		total := sum.ChecksPass + sum.ChecksFail
		if total == 0 {
			return metricActual{Kind: kindFloat, F: 1}, true, nil
		}
		return metricActual{Kind: kindFloat, F: float64(sum.ChecksPass) / float64(total)}, true, nil
	case "dropped_iterations":
		return metricActual{Kind: kindFloat, F: float64(sum.DroppedIterations)}, true, nil
	case "dropped_iterations_rate":
		total := sum.DroppedIterations + sum.Iterations
		if total == 0 {
			return metricActual{Kind: kindFloat, F: 0}, true, nil
		}
		return metricActual{Kind: kindFloat, F: float64(sum.DroppedIterations) / float64(total)}, true, nil
	case "http_req_duration_p50":
		return metricActual{Kind: kindDuration, D: sum.P50}, true, nil
	case "http_req_duration_p90":
		return metricActual{Kind: kindDuration, D: sum.P90}, true, nil
	case "http_req_duration_p95":
		return metricActual{Kind: kindDuration, D: sum.P95}, true, nil
	case "http_req_duration_p99":
		return metricActual{Kind: kindDuration, D: sum.P99}, true, nil
	default:
		return metricActual{}, false, nil
	}
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}

func compareFloat(actual, want float64, op string) bool {
	switch strings.TrimSpace(op) {
	case "<":
		return actual < want
	case "<=":
		return actual <= want
	case ">":
		return actual > want
	case ">=":
		return actual >= want
	case "==":
		return actual == want
	default:
		return false
	}
}

func compareDuration(actual, want time.Duration, op string) bool {
	switch strings.TrimSpace(op) {
	case "<":
		return actual < want
	case "<=":
		return actual <= want
	case ">":
		return actual > want
	case ">=":
		return actual >= want
	case "==":
		return actual == want
	default:
		return false
	}
}
