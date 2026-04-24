package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"benchbug/internal/target"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	seed := flag.Int64("seed", 1, "deterministic seed for /flaky")
	auth := flag.Bool("auth", true, "require bearer token for /users endpoints")
	maxBytes := flag.Int("max-bytes", 8<<20, "max bytes returned by /bytes")
	logRequests := flag.Bool("log-requests", false, "log every HTTP request")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("target listening", "addr", *addr, "auth", *auth)
	err := target.Serve(ctx, *addr, target.Options{
		Logger:      log,
		Seed:        *seed,
		Auth:        *auth,
		MaxBytes:    *maxBytes,
		LogRequests: *logRequests,
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
