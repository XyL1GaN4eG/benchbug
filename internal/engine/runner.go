package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"benchbug/internal/httpx"
	"benchbug/internal/metrics"
	"benchbug/internal/output"
	"benchbug/internal/scenario"

	"github.com/tidwall/gjson"
)

const (
	ExitOK               = 0
	ExitRuntimeError     = 1
	ExitThresholdFailure = 2
)

type RunResult struct {
	Summary  metrics.Summary
	ExitCode int
}

func Run(ctx context.Context, sc *scenario.Scenario, opts Options, stdout io.Writer) (RunResult, error) {
	applyOverrides(sc, opts)
	if err := scenario.Validate(sc); err != nil {
		return RunResult{ExitCode: ExitRuntimeError}, err
	}
	if opts.MaxBody <= 0 {
		opts.MaxBody = 10 << 20
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.Seed == 0 {
		opts.Seed = time.Now().UnixNano()
	}

	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	collector := metrics.NewCollector(metrics.Options{EventsBuffer: 65536})
	snapshots := make(chan metrics.Snapshot, 16)
	collectorStop := make(chan struct{})
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		collector.Run(collectorStop, snapshots)
	}()

	printerDone := make(chan struct{})
	go func() {
		defer close(printerDone)
		consumeOutput(stdout, opts, snapshots)
	}()

	startErr := runScenario(ctx, sc, opts, collector)

	close(collectorStop)
	<-collectorDone
	close(snapshots)
	<-printerDone

	sum := collector.SummaryTopN(20)
	sum.Thresholds = metrics.EvaluateThresholds(thresholdSpecs(sc.Threshold), sum)

	if !opts.JSON {
		output.NewConsole(stdout).OnSummary(sum)
	}
	if opts.JSON {
		outputJSONSummary(stdout, sum)
	}

	exitCode := ExitOK
	if startErr != nil && !errors.Is(startErr, context.Canceled) {
		exitCode = ExitRuntimeError
	}
	for _, th := range sum.Thresholds {
		if !th.OK && exitCode == ExitOK {
			exitCode = ExitThresholdFailure
		}
	}
	return RunResult{Summary: sum, ExitCode: exitCode}, startErr
}

func consumeOutput(w io.Writer, opts Options, snapshots <-chan metrics.Snapshot) {
	console := output.NewConsole(w)
	var jsonOut *output.JSONL
	if opts.JSON {
		jsonOut = output.NewJSONL(nopWriteCloser{w: w})
		defer jsonOut.Close()
	}
	for s := range snapshots {
		if opts.JSON {
			jsonOut.OnSnapshot(s)
			continue
		}
		if !opts.Quiet {
			console.OnSnapshot(s)
		}
	}
}

func outputJSONSummary(w io.Writer, sum metrics.Summary) {
	enc := json.NewEncoder(w)
	_ = enc.Encode(map[string]any{
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
}

type nopWriteCloser struct{ w io.Writer }

func (n nopWriteCloser) Write(p []byte) (int, error) { return n.w.Write(p) }
func (n nopWriteCloser) Close() error                { return nil }

func applyOverrides(sc *scenario.Scenario, opts Options) {
	if opts.Rate > 0 {
		ensureArrival(sc)
		sc.Arrival.Rate = opts.Rate
	}
	if opts.MaxVUs > 0 && sc.Arrival != nil {
		sc.Arrival.MaxVUs = opts.MaxVUs
	}
	if opts.VUs > 0 {
		if sc.Arrival != nil {
			sc.Arrival.MaxVUs = opts.VUs
		} else {
			sc.VUs = opts.VUs
			if len(sc.Stages) > 0 {
				sc.Stages = nil
			}
		}
	}
	if opts.Duration > 0 {
		if sc.Arrival != nil {
			sc.Arrival.Duration.Duration = opts.Duration
		} else {
			sc.Duration.Duration = opts.Duration
			if len(sc.Stages) > 0 {
				sc.Stages = nil
			}
		}
	}
}

func ensureArrival(sc *scenario.Scenario) {
	if sc.Arrival == nil {
		sc.Arrival = &scenario.ArrivalRate{
			Per:      scenario.Duration{Duration: time.Second},
			Duration: sc.Duration,
			MaxVUs:   sc.VUs,
		}
	}
	sc.VUs = 0
	sc.Duration.Duration = 0
	sc.Stages = nil
}

func thresholdSpecs(in []scenario.Threshold) []metrics.ThresholdSpec {
	out := make([]metrics.ThresholdSpec, 0, len(in))
	for _, th := range in {
		out = append(out, metrics.ThresholdSpec{
			Metric: th.Metric,
			Op:     th.Op,
			Value:  fmt.Sprint(th.Value),
		})
	}
	return out
}

func runScenario(ctx context.Context, sc *scenario.Scenario, opts Options, collector *metrics.Collector) error {
	if sc.Arrival != nil {
		return runArrivalRate(ctx, sc, opts, collector)
	}
	return runStages(ctx, sc, opts, collector)
}

func runStages(ctx context.Context, sc *scenario.Scenario, opts Options, collector *metrics.Collector) error {
	stages := sc.Stages
	if len(stages) == 0 {
		stages = []scenario.Stage{{VUs: sc.VUs, Duration: scenario.Duration{Duration: sc.Duration.Duration}}}
	}

	vuCtx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	var active atomic.Int64
	var wg sync.WaitGroup
	stops := map[int]context.CancelFunc{}
	var nextVU int

	stopVU := func(id int) {
		if cancel, ok := stops[id]; ok {
			cancel()
			delete(stops, id)
		}
	}
	startVU := func(id int) {
		vctx, cancel := context.WithCancel(vuCtx)
		stops[id] = cancel
		active.Add(1)
		collector.SetActiveVUs(int(active.Load()))
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				active.Add(-1)
				collector.SetActiveVUs(int(active.Load()))
			}()
			runVU(vctx, id, sc, opts, collector.Events())
		}()
	}

	for _, st := range stages {
		if err := ctx.Err(); err != nil {
			cancelAll()
			wg.Wait()
			return err
		}
		target := st.VUs
		collector.SetStageVUs(target)

		for len(stops) < target {
			nextVU++
			startVU(nextVU)
		}
		for len(stops) > target {
			var id int
			for k := range stops {
				id = k
				break
			}
			stopVU(id)
		}

		timer := time.NewTimer(st.Duration.Duration)
		select {
		case <-ctx.Done():
			timer.Stop()
			cancelAll()
			wg.Wait()
			return ctx.Err()
		case <-timer.C:
		}
	}

	cancelAll()
	wg.Wait()
	return nil
}

func runVU(ctx context.Context, id int, sc *scenario.Scenario, opts Options, events chan<- metrics.Event) {
	client := httpx.NewClient(httpx.ClientOptions{
		Timeout:            opts.Timeout,
		InsecureSkipVerify: opts.Insecure,
	})
	rng := rand.New(rand.NewSource(opts.Seed + int64(id)*7919))
	vars := make(map[string]string, len(sc.Vars)+8)
	for k, v := range sc.Vars {
		vars[k] = v
	}

	var iter int64
	for ctx.Err() == nil {
		iter++
		tctx := scenario.TemplateCtx{VU: id, Iter: iter, Rand: rng}
		runIteration(ctx, client, sc, opts, vars, tctx, events)
		sendEvent(context.Background(), events, metrics.Event{Type: metrics.EventCompletedIteration})
	}
}

func runArrivalRate(ctx context.Context, sc *scenario.Scenario, opts Options, collector *metrics.Collector) error {
	ar := sc.Arrival
	interval := time.Duration(float64(ar.Per.Duration) / ar.Rate)
	if interval < time.Microsecond {
		interval = time.Microsecond
	}

	runCtx, cancel := context.WithTimeout(ctx, ar.Duration.Duration)
	defer cancel()

	client := httpx.NewClient(httpx.ClientOptions{
		Timeout:            opts.Timeout,
		InsecureSkipVerify: opts.Insecure,
	})
	collector.SetStageVUs(ar.MaxVUs)

	slots := make(chan struct{}, ar.MaxVUs)
	var active atomic.Int64
	var wg sync.WaitGroup
	var iter atomic.Int64

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startIteration := func() {
		select {
		case slots <- struct{}{}:
		default:
			sendEvent(runCtx, collector.Events(), metrics.Event{Type: metrics.EventDroppedIteration})
			return
		}

		activeNow := active.Add(1)
		collector.SetActiveVUs(int(activeNow))
		iterationID := iter.Add(1)
		vuID := int(iterationID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				<-slots
				activeNow := active.Add(-1)
				collector.SetActiveVUs(int(activeNow))
			}()

			vars := make(map[string]string, len(sc.Vars)+8)
			for k, v := range sc.Vars {
				vars[k] = v
			}
			rng := rand.New(rand.NewSource(opts.Seed + iterationID*7919))
			tctx := scenario.TemplateCtx{VU: vuID, Iter: iterationID, Rand: rng}
			runIteration(runCtx, client, sc, opts, vars, tctx, collector.Events())
			sendEvent(context.Background(), collector.Events(), metrics.Event{Type: metrics.EventCompletedIteration})
		}()
	}

	startIteration()
	for {
		select {
		case <-runCtx.Done():
			wg.Wait()
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return runCtx.Err()
		case <-ticker.C:
			startIteration()
		}
	}
}

func runIteration(ctx context.Context, client *http.Client, sc *scenario.Scenario, opts Options, vars map[string]string, tctx scenario.TemplateCtx, events chan<- metrics.Event) {
	for _, step := range sc.Steps {
		if ctx.Err() != nil {
			return
		}
		runStep(ctx, client, sc, step, opts, vars, tctx, events)
		if step.ThinkTime.Duration > 0 {
			timer := time.NewTimer(step.ThinkTime.Duration)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func runStep(ctx context.Context, client *http.Client, sc *scenario.Scenario, step scenario.Step, opts Options, vars map[string]string, tctx scenario.TemplateCtx, events chan<- metrics.Event) {
	reqSpec, key, err := buildSpec(sc, step, vars, tctx, opts)
	if err != nil {
		sendRequestError(ctx, events, key, 0, 0, 0, err)
		return
	}

	timeout := reqSpec.Timeout
	if timeout <= 0 {
		timeout = opts.Timeout
	}
	reqCtx, cancel := httpx.WithTimeout(ctx, timeout)
	defer cancel()

	req, out, err := httpx.BuildRequest(reqCtx, reqSpec)
	if err != nil {
		sendRequestError(ctx, events, key, 0, 0, 0, err)
		return
	}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		sendRequestError(ctx, events, key, elapsed, 0, len(out), err)
		return
	}

	rr, readErr := httpx.ReadResponse(resp, opts.MaxBody)
	if readErr != nil {
		if ctx.Err() != nil {
			return
		}
		sendRequestError(ctx, events, key, elapsed, resp.StatusCode, len(out), readErr)
		return
	}

	sendEvent(ctx, events, metrics.Event{
		Type:     metrics.EventRequest,
		Key:      key,
		Duration: elapsed,
		Status:   rr.Status,
		BytesIn:  len(rr.Body),
		BytesOut: len(out),
	})
	applyExtracts(step.Extract, rr, vars)
	runChecks(ctx, events, key, step.Checks, rr)
}

func sendRequestError(ctx context.Context, events chan<- metrics.Event, key metrics.Key, d time.Duration, status, bytesOut int, err error) {
	sendEvent(ctx, events, metrics.Event{
		Type:     metrics.EventRequest,
		Key:      key,
		Duration: d,
		Status:   status,
		Err:      err.Error(),
		BytesOut: bytesOut,
	})
}

func buildSpec(sc *scenario.Scenario, step scenario.Step, vars map[string]string, tctx scenario.TemplateCtx, opts Options) (httpx.RequestSpec, metrics.Key, error) {
	key := metrics.Key{
		Group:  step.Group,
		Step:   step.Name,
		Method: strings.ToUpper(step.Request.Method),
		URL:    metricURL(sc.BaseURL, step.Request.URL),
	}
	headers := make(map[string]string, len(sc.Defaults.Headers)+len(step.Request.Headers))
	defaultHeaders, err := expandStringMap(sc.Defaults.Headers, vars, tctx)
	if err != nil {
		return httpx.RequestSpec{}, key, fmt.Errorf("default headers: %w", err)
	}
	stepHeaders, err := expandStringMap(step.Request.Headers, vars, tctx)
	if err != nil {
		return httpx.RequestSpec{}, key, fmt.Errorf("headers: %w", err)
	}
	for k, v := range defaultHeaders {
		headers[k] = v
	}
	for k, v := range stepHeaders {
		headers[k] = v
	}
	u, err := scenario.Expand(step.Request.URL, vars, tctx)
	if err != nil {
		return httpx.RequestSpec{}, key, fmt.Errorf("url template: %w", err)
	}
	body, err := scenario.Expand(step.Request.Body, vars, tctx)
	if err != nil {
		return httpx.RequestSpec{}, key, fmt.Errorf("body template: %w", err)
	}
	jsonBody, err := expandJSON(step.Request.JSON, vars, tctx)
	if err != nil {
		return httpx.RequestSpec{}, key, fmt.Errorf("json body template: %w", err)
	}
	form, err := expandStringMap(step.Request.Form, vars, tctx)
	if err != nil {
		return httpx.RequestSpec{}, key, fmt.Errorf("form: %w", err)
	}
	timeout := step.Request.Timeout.Duration
	if timeout <= 0 {
		timeout = sc.Defaults.Timeout.Duration
	}
	if timeout <= 0 {
		timeout = opts.Timeout
	}
	return httpx.RequestSpec{
		Method:  step.Request.Method,
		BaseURL: sc.BaseURL,
		URL:     u,
		Headers: headers,
		Body:    body,
		JSON:    jsonBody,
		Form:    form,
		Timeout: timeout,
	}, key, nil
}

func expandStringMap(in map[string]string, vars map[string]string, tctx scenario.TemplateCtx) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		ev, err := scenario.Expand(v, vars, tctx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = ev
	}
	return out, nil
}

func expandJSON(v any, vars map[string]string, tctx scenario.TemplateCtx) (any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case string:
		return scenario.Expand(x, vars, tctx)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			ev, err := expandJSON(item, vars, tctx)
			if err != nil {
				return nil, err
			}
			out[i] = ev
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			ev, err := expandJSON(item, vars, tctx)
			if err != nil {
				return nil, err
			}
			out[k] = ev
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			ev, err := expandJSON(item, vars, tctx)
			if err != nil {
				return nil, err
			}
			out[fmt.Sprint(k)] = ev
		}
		return out, nil
	default:
		return v, nil
	}
}

func applyExtracts(ex scenario.Extract, resp *httpx.Response, vars map[string]string) {
	for name, path := range ex.JSONPath {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
			continue
		}
		res := gjson.GetBytes(resp.Body, path)
		if res.Exists() {
			vars[name] = res.String()
		}
	}
	for name, header := range ex.Header {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(header) == "" {
			continue
		}
		vars[name] = resp.Headers.Get(header)
	}
}

func runChecks(ctx context.Context, events chan<- metrics.Event, key metrics.Key, checks []scenario.Check, resp *httpx.Response) {
	for _, ch := range checks {
		if len(ch.StatusIn) > 0 {
			ok := false
			for _, st := range ch.StatusIn {
				if resp.Status == st {
					ok = true
					break
				}
			}
			sendEvent(ctx, events, metrics.Event{Type: metrics.EventCheck, Key: key, CheckOK: ok})
		}
		if ch.JSONPathExists != "" {
			sendEvent(ctx, events, metrics.Event{
				Type:    metrics.EventCheck,
				Key:     key,
				CheckOK: gjson.GetBytes(resp.Body, ch.JSONPathExists).Exists(),
			})
		}
		if ch.JSONPathEq != nil {
			actual := gjson.GetBytes(resp.Body, ch.JSONPathEq.Path)
			want := fmt.Sprint(ch.JSONPathEq.Value)
			sendEvent(ctx, events, metrics.Event{
				Type:    metrics.EventCheck,
				Key:     key,
				CheckOK: actual.Exists() && actual.String() == want,
			})
		}
		for header, want := range ch.HeaderEq {
			sendEvent(ctx, events, metrics.Event{
				Type:    metrics.EventCheck,
				Key:     key,
				CheckOK: resp.Headers.Get(header) == fmt.Sprint(want),
			})
		}
	}
}

func sendEvent(ctx context.Context, events chan<- metrics.Event, ev metrics.Event) {
	select {
	case events <- ev:
	case <-ctx.Done():
	}
}

func metricURL(baseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	pathOnly := raw
	if i := strings.IndexByte(pathOnly, '?'); i >= 0 {
		pathOnly = pathOnly[:i]
	}
	if strings.HasPrefix(pathOnly, "http://") || strings.HasPrefix(pathOnly, "https://") {
		u, err := url.Parse(pathOnly)
		if err == nil {
			u.RawQuery = ""
			u.Fragment = ""
			return u.String()
		}
		return pathOnly
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return pathOnly
	}
	if !strings.HasPrefix(pathOnly, "/") {
		pathOnly = "/" + pathOnly
	}
	return baseURL + pathOnly
}
