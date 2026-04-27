package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"benchbug/internal/scenario"
)

func TestRunHTTPScenario(t *testing.T) {
	var seenPosts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "abc"})
		case "/echo":
			seenPosts++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	sc := &scenario.Scenario{
		Name:     "integration",
		BaseURL:  srv.URL,
		VUs:      1,
		Duration: scenario.Duration{Duration: 250 * time.Millisecond},
		Defaults: scenario.Defaults{Timeout: scenario.Duration{Duration: time.Second}},
		Vars:     map[string]string{"client": "demo-${__vu}"},
		Steps: []scenario.Step{
			{
				Name:  "token",
				Group: "auth",
				Request: scenario.Request{
					Method: "GET",
					URL:    "/token?client=${client}",
				},
				Extract: scenario.Extract{JSONPath: map[string]string{"token": "token"}},
				Checks:  []scenario.Check{{StatusIn: []int{200}, JSONPathExists: "token"}},
			},
			{
				Name:  "echo",
				Group: "api",
				Request: scenario.Request{
					Method: "POST",
					URL:    "/echo",
					JSON:   map[string]any{"token": "${token}"},
				},
				Checks: []scenario.Check{{StatusIn: []int{200}, JSONPathEq: &scenario.JSONPathEq{Path: "ok", Value: true}}},
			},
		},
		Threshold: []scenario.Threshold{
			{Metric: "http_req_failed_rate", Op: "<", Value: 0.01},
			{Metric: "checks_pass_rate", Op: ">", Value: 0.99},
		},
	}

	var out bytes.Buffer
	res, err := Run(context.Background(), sc, Options{Timeout: time.Second, Quiet: true}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("exit code = %d, summary = %+v", res.ExitCode, res.Summary)
	}
	if res.Summary.Requests == 0 || res.Summary.Fails != 0 || seenPosts == 0 {
		t.Fatalf("bad summary: %+v seenPosts=%d", res.Summary, seenPosts)
	}
	if res.Summary.ChecksFail != 0 || res.Summary.ChecksPass == 0 {
		t.Fatalf("bad checks: %+v", res.Summary)
	}
}

func TestRunArrivalRateDropsIterationsWhenMaxVUsIsTooLow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer srv.Close()

	sc := &scenario.Scenario{
		Name:    "arrival",
		BaseURL: srv.URL,
		Arrival: &scenario.ArrivalRate{
			Rate:     50,
			Per:      scenario.Duration{Duration: time.Second},
			Duration: scenario.Duration{Duration: 500 * time.Millisecond},
			MaxVUs:   1,
		},
		Defaults: scenario.Defaults{Timeout: scenario.Duration{Duration: time.Second}},
		Steps: []scenario.Step{{
			Name:  "slow",
			Group: "open",
			Request: scenario.Request{
				Method: "GET",
				URL:    "/slow",
			},
			Checks: []scenario.Check{{StatusIn: []int{200}}},
		}},
		Threshold: []scenario.Threshold{
			{Metric: "dropped_iterations", Op: ">", Value: 0},
		},
	}

	var out bytes.Buffer
	res, err := Run(context.Background(), sc, Options{Timeout: time.Second, Quiet: true}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.DroppedIterations == 0 {
		t.Fatalf("expected dropped iterations, summary = %+v", res.Summary)
	}
	if res.Summary.Iterations == 0 || res.Summary.Requests == 0 {
		t.Fatalf("expected completed iterations, summary = %+v", res.Summary)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("exit code = %d, summary = %+v", res.ExitCode, res.Summary)
	}
}
