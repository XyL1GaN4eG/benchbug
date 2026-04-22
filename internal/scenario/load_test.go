package scenario

import (
	"strings"
	"testing"
	"time"
)

func TestValidateArrivalRate(t *testing.T) {
	sc := &Scenario{
		Name: "arrival",
		Arrival: &ArrivalRate{
			Rate:     10,
			Per:      Duration{Duration: time.Second},
			Duration: Duration{Duration: time.Second},
			MaxVUs:   2,
		},
		Steps: []Step{{
			Name:    "health",
			Request: Request{Method: "GET", URL: "/health"},
		}},
	}
	if err := Validate(sc); err != nil {
		t.Fatal(err)
	}
}

func TestValidateArrivalRateRejectsClosedModelFields(t *testing.T) {
	sc := &Scenario{
		Name:     "bad",
		VUs:      1,
		Duration: Duration{Duration: time.Second},
		Arrival: &ArrivalRate{
			Rate:     10,
			Per:      Duration{Duration: time.Second},
			Duration: Duration{Duration: time.Second},
			MaxVUs:   2,
		},
		Steps: []Step{{
			Name:    "health",
			Request: Request{Method: "GET", URL: "/health"},
		}},
	}
	err := Validate(sc)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "arrival_rate cannot be combined") {
		t.Fatalf("unexpected error: %v", err)
	}
}
