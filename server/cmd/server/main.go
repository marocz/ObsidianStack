package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("obsidianstack-server starting")

	// TODO(T008): start gRPC receiver
	// TODO(T009): start REST API
	// TODO(T010): start WebSocket hub

	<-ctx.Done()
	slog.Info("obsidianstack-server shutting down")
}
