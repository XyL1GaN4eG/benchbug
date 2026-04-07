package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Scenario struct {
	Name     string `yaml:"name"`
	BaseURL  string `yaml:"base_url"`
	VUs      int    `yaml:"vus"`
	Duration string `yaml:"duration"`
	Tasks    []Task `yaml:"tasks"`
}

type Task struct {
	Name   string `yaml:"name"`
	Method string `yaml:"method"`
	URL    string `yaml:"url"`
}

func main() {
	file := flag.String("f", "", "scenario file")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "missing -f")
		os.Exit(1)
	}
	sc, err := loadScenario(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := runScenario(sc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadScenario(path string) (*Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sc Scenario
	if err := yaml.Unmarshal(b, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

func runScenario(sc *Scenario) error {
	d, _ := time.ParseDuration(sc.Duration)
	stop := time.After(d)
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
					for _, task := range sc.Tasks {
						resp, err := http.Get(sc.BaseURL + task.URL)
						if err != nil {
							fmt.Fprintln(os.Stderr, err)
							continue
						}
						fmt.Println(id, task.Name, resp.StatusCode)
						resp.Body.Close()
					}
				}
			}
		}(i + 1)
	}
	wg.Wait()
	return nil
}
