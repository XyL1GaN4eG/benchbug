package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"benchbug/internal/engine"
	"benchbug/internal/scenario"
)

func main() { os.Exit(run(os.Args[1:])) }

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
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage:
  benchbug validate -f scenario.yaml
  benchbug run -f scenario.yaml
`)
}

func validateCommand(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("f", "", "scenario file")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(os.Stderr, "validate: -f is required")
		return 1
	}
	if _, err := scenario.LoadFile(*file); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s: ok\n", *file)
	return 0
}

func runCommand(args []string) int {
	opts := engine.Options{}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.File, "f", "", "scenario file")
	fs.IntVar(&opts.VUs, "vus", 0, "override VUs")
	fs.DurationVar(&opts.Duration, "duration", 0, "override duration")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if opts.File == "" {
		fmt.Fprintln(os.Stderr, "run: -f is required")
		return 1
	}
	sc, err := scenario.LoadFile(opts.File)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, err = engine.Run(context.Background(), sc, opts, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
