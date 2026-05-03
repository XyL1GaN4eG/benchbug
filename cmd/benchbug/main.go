package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"benchbug/internal/engine"
	"benchbug/internal/scenario"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	switch args[0] {
	case "run":
		return runCommand(args[1:])
	case "validate":
		return validateCommand(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		usage()
		return 1
	}
}

func usage() {
	_, _ = fmt.Fprint(os.Stderr, `benchbug is a local HTTP load testing CLI.

Usage:
  benchbug validate -f scenario.yaml
  benchbug run -f scenario.yaml [flags]

Run flags:
  -vus N             override scenario VUs and ignore stages
  -duration 30s      override scenario duration and ignore stages
  -arrival-rate N    run N new iterations per second (open model)
  -max-vus N         max concurrent iterations for arrival-rate mode
  -timeout 10s       default request timeout
  -seed N            deterministic seed for builtins like ${__rand_int(1,10)}
  -json              print JSONL snapshots + summary to stdout
  -quiet             print only final summary
  -insecure          skip TLS certificate verification
  -max-body BYTES    max response body bytes read per request (default 10485760)

`)
}

func validateCommand(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("f", "", "scenario file")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *file == "" {
		_, _ = fmt.Fprintln(os.Stderr, "validate: -f is required")
		return 1
	}
	if _, err := scenario.LoadFile(*file); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, _ = fmt.Printf("%s: ok\n", *file)
	return 0
}

func runCommand(args []string) int {
	opts := &engine.Options{
		Timeout: 10 * time.Second,
		MaxBody: 10 << 20,
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.File, "f", "", "scenario file")
	fs.IntVar(&opts.VUs, "vus", 0, "override VUs and ignore stages")
	fs.DurationVar(&opts.Duration, "duration", 0, "override duration and ignore stages")
	fs.Float64Var(&opts.Rate, "arrival-rate", 0, "run N new iterations per second")
	fs.IntVar(&opts.MaxVUs, "max-vus", 0, "max concurrent iterations for arrival-rate mode")
	fs.Int64Var(&opts.Seed, "seed", 0, "deterministic seed")
	fs.BoolVar(&opts.JSON, "json", false, "print JSONL snapshots + summary")
	fs.BoolVar(&opts.Quiet, "quiet", false, "print only final summary")
	fs.DurationVar(&opts.Timeout, "timeout", opts.Timeout, "default request timeout")
	fs.BoolVar(&opts.Insecure, "insecure", false, "skip TLS certificate verification")
	fs.Int64Var(&opts.MaxBody, "max-body", opts.MaxBody, "max response body bytes read per request")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if opts.File == "" {
		_, _ = fmt.Fprintln(os.Stderr, "run: -f is required")
		return 1
	}

	sc, err := scenario.LoadFile(opts.File)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result, err := engine.Run(context.Background(), sc, *opts, os.Stdout)
	if err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
	return result.ExitCode
}
