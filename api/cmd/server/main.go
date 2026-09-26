package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/api"
	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

// shutdownTimeout is how long in-flight requests and ingestion jobs get to
// finish after SIGTERM. Hosting platforms typically allow ~30s before a
// hard kill; interrupted jobs are recorded as pending and resume on the
// next start.
const shutdownTimeout = 25 * time.Second

func main() {

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := newLogger(cfg.LogFormat)

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Start(ctx, shutdownTimeout); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// newLogger returns JSON logs for production (machine-parseable by log
// aggregators) and human-readable text locally.
func newLogger(format string) *slog.Logger {
	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
