package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
	"github.com/obsidianstack/obsidianstack/agent/internal/shipper"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("obsidianstack-agent starting", "config", *configPath)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}
	slog.Info("config loaded",
		"server_endpoint", cfg.Agent.ServerEndpoint,
		"sources", len(cfg.Agent.Sources),
		"scrape_interval", cfg.Agent.ScrapeInterval,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Hold the latest config in a mutex-protected pointer so hot-reload is safe.
	var mu sync.RWMutex
	current := cfg

	// Watch config file for hot-reload in background.
	go func() {
		if err := config.Watch(ctx, *configPath, func(updated *config.Config) {
			mu.Lock()
			current = updated
			mu.Unlock()
			slog.Info("config hot-reloaded",
				"sources", len(updated.Agent.Sources),
			)
		}); err != nil {
			slog.Error("config watcher stopped", "err", err)
		}
	}()

	// Start the gRPC shipper — runs until ctx is cancelled.
	ship := shipper.New(cfg.Agent)
	go ship.Run(ctx)

	// TODO(T003-T005): build scraper instances from cfg.Agent.Sources,
	// feed them through compute.Engine, then call ship.Ship(result)
	// on each tick of cfg.Agent.ScrapeInterval.

	<-ctx.Done()
	slog.Info("obsidianstack-agent shutting down")
}
