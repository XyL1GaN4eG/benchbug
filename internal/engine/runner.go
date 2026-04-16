package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"benchbug/internal/httpx"
	"benchbug/internal/metrics"
	"benchbug/internal/scenario"
)

type RunResult struct {
	Summary  metrics.Summary
	ExitCode int
}

func Run(ctx context.Context, sc *scenario.Scenario, opts Options, stdout io.Writer) (RunResult, error) {
	if opts.VUs > 0 {
		sc.VUs = opts.VUs
	}
	if opts.Duration > 0 {
		sc.Duration.Duration = opts.Duration
	}
	collector := metrics.NewCollector()
	stop := time.After(sc.Duration.Duration)
	var wg sync.WaitGroup
	for i := 0; i < sc.VUs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-stop:
					return
				default:
					for _, step := range sc.Steps {
						u, err := scenario.Expand(step.URL, sc.Vars, scenario.TemplateCtx{VU: id, Iter: 1})
						if err != nil {
							fmt.Fprintln(stdout, err)
							continue
						}
						req, err := httpx.BuildRequest(ctx, httpx.RequestSpec{Method: step.Method, BaseURL: sc.BaseURL, URL: u})
						if err != nil {
							fmt.Fprintln(stdout, err)
							continue
						}
						resp, err := http.DefaultClient.Do(req)
						if err != nil {
							collector.Add(metrics.Event{Err: err.Error()})
							continue
						}
						collector.Add(metrics.Event{Status: resp.StatusCode})
						resp.Body.Close()
					}
				}
			}
		}(i + 1)
	}
	wg.Wait()
	sum := collector.Summary()
	fmt.Fprintf(stdout, "summary requests=%d fails=%d\n", sum.Requests, sum.Fails)
	return RunResult{Summary: sum}, nil
}
