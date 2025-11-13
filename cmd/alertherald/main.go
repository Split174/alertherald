package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Split174/alert-herald/internal/config"
	"github.com/Split174/alert-herald/internal/engine"
	"github.com/Split174/alert-herald/internal/notification"
	"github.com/Split174/alert-herald/internal/server"
	"github.com/Split174/alert-herald/internal/store"
)

func main() {
	// --- 1. parsing ---
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	configPath := flag.String("config.path", "./config", "Path to the configuration file or directory.")
	dbPath := flag.String("db.path", "./alertherald.db", "Path to the SQLite database file.")
	telegramToken := flag.String("telegram.token", "", "Telegram Bot Token.")
	listenAddr := flag.String("web.listen-address", ":8080", "Address to listen on for web interface and API.")
	flag.Parse()

	if *telegramToken == "" {
		slog.Error("Telegram token is required. Please provide it using -telegram.token flag or an environment variable.")
		os.Exit(1)
	}

	// --- 2. init db, telegram, config ---
	db, err := store.NewStore(*dbPath)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	slog.Info("Database initialized successfully", "path", *dbPath)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load initial configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("Configuration loaded successfully", "users", len(cfg.Users), "schedules", len(cfg.Schedules), "policies", len(cfg.EscalationPolicies))

	telegramNotifier := notification.NewTelegramNotifier(*telegramToken)

	escalationEngine, err := engine.NewEngine(db, telegramNotifier, cfg)
	if err != nil {
		slog.Error("Failed to create escalation engine", "error", err)
		os.Exit(1)
	}

	// --- 3. Hotreload by SIGHUP ---
	reloadChan := make(chan os.Signal, 1)
	signal.Notify(reloadChan, syscall.SIGHUP)
	go func() {
		for range reloadChan {
			slog.Info("Received SIGHUP, reloading configuration...")
			newCfg, err := config.Load(*configPath)
			if err != nil {
				slog.Error("Failed to reload configuration", "error", err)
				continue
			}
			escalationEngine.UpdateConfig(newCfg)
			slog.Info("Configuration reloaded successfully")
		}
	}()

	// --- 4. Run webserver ---
	srv := server.NewServer(*listenAddr, db, escalationEngine)

	go func() {
		slog.Info("Starting server", "address", *listenAddr)
		if err := srv.Start(); err != nil {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// --- 5. Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	escalationEngine.Stop()
	slog.Info("Server exited gracefully")
}
