package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/atharva-3105/KnowYourRepo/internal/api"
	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	slog.SetDefault(logger)

	cfg, err := config.Load()

	if err !=nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	srv := api.NewServer(cfg, logger)

	slog.Info("starting server", "addr", cfg.Server.Addr())
	if err := srv.Start(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}