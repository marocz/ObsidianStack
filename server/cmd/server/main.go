package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
	"github.com/obsidianstack/obsidianstack/server/internal/alerts"
	"github.com/obsidianstack/obsidianstack/server/internal/api"
	"github.com/obsidianstack/obsidianstack/server/internal/auth"
	"github.com/obsidianstack/obsidianstack/server/internal/config"
	"github.com/obsidianstack/obsidianstack/server/internal/receiver"
	"github.com/obsidianstack/obsidianstack/server/internal/store"
	"github.com/obsidianstack/obsidianstack/server/internal/ws"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	uiDir := flag.String("ui-dir", "", "serve the React UI static files from this directory (e.g. ui/dist); leave empty to disable")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("obsidianstack-server starting", "config", *configPath)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	slog.Info("config loaded",
		"grpc_port", cfg.Server.GRPCPort,
		"http_port", cfg.Server.HTTPPort,
		"auth_mode", cfg.Server.Auth.Mode,
		"snapshot_ttl", cfg.Server.Snapshot.TTL,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Snapshot store with background TTL eviction.
	st := store.New(cfg.Server.Snapshot.TTL)
	go st.Run(ctx)

	// Alerts engine — evaluates rules on every incoming snapshot.
	alertEngine := alerts.New(cfg.Server.Alerts)

	// gRPC server with optional API key authentication interceptor.
	interceptor := auth.APIKeyInterceptor(
		cfg.Server.Auth.Mode,
		cfg.Server.Auth.EffectiveHeader(),
		cfg.Server.Auth.Key(),
	)
	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	pb.RegisterSnapshotServiceServer(grpcSrv, receiver.New(st, alertEngine))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		slog.Error("failed to listen on gRPC port",
			"port", cfg.Server.GRPCPort, "err", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("gRPC receiver listening", "port", cfg.Server.GRPCPort)
		if err := grpcSrv.Serve(lis); err != nil {
			slog.Error("gRPC server stopped", "err", err)
		}
	}()

	// WebSocket hub — broadcasts snapshots to UI clients every 5 seconds.
	hub := ws.New(st, 5*time.Second)
	go hub.Run(ctx)

	// Combined HTTP server: REST API + WebSocket hub on HTTPPort.
	httpMux := http.NewServeMux()
	httpMux.Handle("/api/", api.New(st, alertEngine))
	httpMux.Handle("/ws/stream", hub)

	// Optional: serve the pre-built React UI from a local directory.
	// Usage:  ./bin/obsidianstack-server -config config/server.yaml -ui-dir ui/dist
	// The "/" catch-all serves index.html for any unknown path (SPA routing).
	if *uiDir != "" {
		fs := http.FileServer(http.Dir(*uiDir))
		httpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: if the requested file doesn't exist, serve index.html.
			path := *uiDir + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, *uiDir+"/index.html")
				return
			}
			fs.ServeHTTP(w, r)
		})
		slog.Info("serving UI static files", "dir", *uiDir)
	}

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler: httpMux,
	}
	go func() {
		slog.Info("HTTP server listening", "port", cfg.Server.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server stopped", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("obsidianstack-server shutting down")
	grpcSrv.GracefulStop()
	httpSrv.Shutdown(context.Background()) //nolint:errcheck
}
