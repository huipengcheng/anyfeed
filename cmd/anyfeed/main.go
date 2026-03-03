// Package main is the entry point for the Anyfeed application.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/server"
	"github.com/huipeng/anyfeed/internal/source"
	"github.com/huipeng/anyfeed/internal/source/email"
	"github.com/huipeng/anyfeed/internal/source/rss"
	"github.com/huipeng/anyfeed/internal/source/web"
	"github.com/huipeng/anyfeed/internal/store"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/example.yaml", "Path to configuration file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Setup logging
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("starting Anyfeed", "config", *configPath)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("configuration loaded",
		"feeds", len(cfg.Feeds),
		"outputs", len(cfg.Output),
		"port", cfg.Server.Port,
	)

	// Initialize store
	st, err := store.NewSQLiteStore(cfg.Storage.Path)
	if err != nil {
		slog.Error("failed to initialize store", "error", err)
		os.Exit(1)
	}
	defer st.Close()
	slog.Info("store initialized", "path", cfg.Storage.Path)

	// Create entry handler for saving to store
	entryHandler := func(entries []*source.Entry) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := st.SaveEntries(ctx, entries); err != nil {
			slog.Error("failed to save entries", "error", err)
			return
		}

		// Cleanup old entries for each source
		sourceNames := make(map[string]bool)
		for _, e := range entries {
			sourceNames[e.SourceName] = true
		}
		for sourceName := range sourceNames {
			if err := st.DeleteOldEntries(ctx, sourceName, cfg.Storage.MaxItemsPerFeed); err != nil {
				slog.Warn("failed to cleanup old entries", "source", sourceName, "error", err)
			}
		}
	}

	// Initialize source manager
	mgr := source.NewManager(entryHandler)

	// Register RSS sources
	for _, feedCfg := range cfg.GetFeedsByType(config.FeedTypeRSS) {
		src := rss.New(feedCfg)
		if err := mgr.Register(src); err != nil {
			slog.Error("failed to register RSS source", "name", feedCfg.Name, "error", err)
		}
	}

	// Register Web sources
	for _, feedCfg := range cfg.GetFeedsByType(config.FeedTypeWeb) {
		src := web.New(feedCfg, st)
		if err := mgr.Register(src); err != nil {
			slog.Error("failed to register Web source", "name", feedCfg.Name, "error", err)
		}
	}

	// Register Email sources (all share the same SMTP server)
	for _, feedCfg := range cfg.GetFeedsByType(config.FeedTypeEmail) {
		src, err := email.New(feedCfg, cfg.Server.SMTPPort)
		if err != nil {
			slog.Error("failed to create Email source", "name", feedCfg.Name, "error", err)
			continue
		}
		if err := mgr.Register(src); err != nil {
			slog.Error("failed to register Email source", "name", feedCfg.Name, "error", err)
		}
	}

	// Create and start HTTP server
	srv := server.New(cfg, st)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start source manager
	if err := mgr.Start(ctx); err != nil {
		slog.Error("failed to start source manager", "error", err)
		os.Exit(1)
	}

	// Start HTTP server
	if err := srv.Start(ctx); err != nil {
		slog.Error("failed to start HTTP server", "error", err)
		os.Exit(1)
	}

	slog.Info("Anyfeed is running",
		"http", fmt.Sprintf("http://localhost:%d", cfg.Server.Port),
		"feeds", len(cfg.Feeds),
	)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop source manager
	if err := mgr.Stop(); err != nil {
		slog.Error("failed to stop source manager", "error", err)
	}

	// Stop HTTP server
	if err := srv.Stop(shutdownCtx); err != nil {
		slog.Error("failed to stop HTTP server", "error", err)
	}

	slog.Info("Anyfeed shutdown complete")
}
