package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	url := flag.String("url", "", "url to hit")
	vus := flag.Int("vus", 1, "virtual users")
	duration := flag.Duration("duration", 10*time.Second, "run duration")
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "missing -url")
		os.Exit(1)
	}

	stop := time.After(*duration)
	var wg sync.WaitGroup
	for i := 0; i < *vus; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					resp, err := http.Get(*url)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						continue
					}
					fmt.Println("vu", id, resp.StatusCode)
					resp.Body.Close()
				}
			}
		}(i + 1)
	}
	wg.Wait()
}
