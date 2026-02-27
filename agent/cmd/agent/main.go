package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/obsidianstack/obsidianstack/agent/internal/compute"
	"github.com/obsidianstack/obsidianstack/agent/internal/config"
	"github.com/obsidianstack/obsidianstack/agent/internal/scraper"
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

	// Build scraper + engine instances from the initial config.
	// Hot-reload updates logging only; rebuilding scrapers on reload is T-future.
	type pipeline struct {
		src    config.Source
		s      scraper.Scraper
		engine *compute.Engine
	}
	var pipelines []pipeline
	for _, src := range cfg.Agent.Sources {
		s, err := scraper.New(src)
		if err != nil {
			slog.Error("skipping source — could not build scraper", "source", src.ID, "err", err)
			continue
		}
		pipelines = append(pipelines, pipeline{src: src, s: s, engine: compute.NewEngine()})
		slog.Info("registered source", "id", src.ID, "type", src.Type, "endpoint", src.Endpoint)
	}

	if len(pipelines) == 0 {
		slog.Warn("no sources configured — agent will idle")
	}

	// Watch config file for hot-reload (logs only in this phase).
	go func() {
		if err := config.Watch(ctx, *configPath, func(updated *config.Config) {
			slog.Info("config hot-reloaded", "sources", len(updated.Agent.Sources))
		}); err != nil {
			slog.Error("config watcher stopped", "err", err)
		}
	}()

	// Start the gRPC shipper — runs until ctx is cancelled.
	ship := shipper.New(cfg.Agent)
	go ship.Run(ctx)

	// Scrape loop: poll every ScrapeInterval, compute strength score, ship.
	go func() {
		ticker := time.NewTicker(cfg.Agent.ScrapeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				for _, p := range pipelines {
					res, err := p.s.Scrape(ctx)
					if err != nil {
						slog.Warn("scrape error", "source", p.src.ID, "err", err)
						continue
					}
					if result := p.engine.Process(res, t); result != nil {
						ship.Ship(result)
						slog.Debug("shipped snapshot",
							"source", p.src.ID,
							"state", result.State,
							"score", result.StrengthScore,
						)
					}
				}
			}
		}
	}()

	<-ctx.Done()
	slog.Info("obsidianstack-agent shutting down")
}
