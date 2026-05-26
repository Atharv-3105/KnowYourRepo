package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/atharva-3105/KnowYourRepo/internal/api"
	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

func main() {
	
	cfg, err  := config.Load()

	logger := slog.New(
		slog.NewTextHandler(
			os.Stdout,
			nil,
		),
	)

	server, err := api.NewServer(cfg, logger)

	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {

		log.Fatalf("server failed: %v", err)
	}
}