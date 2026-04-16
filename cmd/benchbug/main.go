package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"benchbug/internal/engine"
	"benchbug/internal/scenario"
)

func main() {
	opts := engine.Options{}
	flag.StringVar(&opts.File, "f", "", "scenario file")
	flag.IntVar(&opts.VUs, "vus", 0, "override VUs")
	flag.DurationVar(&opts.Duration, "duration", 0, "override duration")
	flag.Parse()
	if strings.TrimSpace(opts.File) == "" {
		fmt.Fprintln(os.Stderr, "usage: benchbug -f scenario.yaml")
		os.Exit(1)
	}
	sc, err := scenario.LoadFile(opts.File)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := engine.Run(context.Background(), sc, opts, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = time.Second
}
