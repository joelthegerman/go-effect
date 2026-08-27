// Command server is the Todos HTTP service. This file is wiring only — it holds
// no business logic. Startup: load config -> connect Postgres -> migrate ->
// serve, with graceful shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"agentic-sandbox/internal/api"
	"agentic-sandbox/internal/config"
	"agentic-sandbox/internal/shell"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "run database migrations and exit")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger, *migrateOnly); err != nil {
		logger.Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger, migrateOnly bool) error {
	cfg := config.Load()

	// Signals cancel this context, which drives graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := shell.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to database")

	if err := shell.Migrate(ctx, pool); err != nil {
		return err
	}
	logger.Info("migrations applied")

	if migrateOnly {
		return nil
	}

	repo := shell.NewRepo(pool)
	server := api.NewServer(repo, logger)

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      server.Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Serve in the background; a serve error cancels the wait below.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("listening", slog.String("addr", cfg.Addr))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("stopped cleanly")
	return nil
}
