package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"benchbug/internal/scenario"
)

func main() {
	file := flag.String("f", "", "scenario file")
	flag.Parse()
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(os.Stderr, "usage: benchbug -f scenario.yaml")
		os.Exit(1)
	}
	sc, err := scenario.LoadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := runScenario(sc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runScenario(sc *scenario.Scenario) error {
	stop := time.After(sc.Duration.Duration)
	var wg sync.WaitGroup
	for i := 0; i < sc.VUs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					for _, step := range sc.Steps {
						resp, err := http.Get(sc.BaseURL + step.URL)
						if err != nil {
							fmt.Fprintln(os.Stderr, err)
							continue
						}
						fmt.Printf("vu=%d task=%s status=%d\n", id, step.Name, resp.StatusCode)
						resp.Body.Close()
					}
				}
			}
		}(i + 1)
	}
	wg.Wait()
	return nil
}
