// Command server is the Todos HTTP service. This file is wiring only — it holds
// no business logic. Startup: load config -> connect Postgres -> migrate ->
// serve, with graceful shutdown on SIGINT/SIGTERM.
//
// It is the composition root: the one place that knows every feature, wiring
// each feature's shell (repository, policies, migrations) and HTTP handlers to
// the framework kernel. Adding a feature adds a block here.
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

	todosapi "agentic-sandbox/internal/api/todos"
	"agentic-sandbox/internal/kernel"
	todosstore "agentic-sandbox/internal/shell/todos"
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
	cfg := kernel.Load()

	// Signals cancel this context, which drives graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := kernel.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to database")

	// Each feature owns its migrations; the kernel just applies them.
	if err := kernel.Migrate(ctx, pool, todosstore.Migrations()); err != nil {
		return err
	}
	logger.Info("migrations applied")

	if migrateOnly {
		return nil
	}

	// --- wire the todos feature -------------------------------------------
	todosRepo := todosstore.NewRepo(pool)
	todosPipe := kernel.NewPipeline(todosRepo, logger, todosstore.Policies()...)
	todosHandlers := todosapi.NewHandlers(todosRepo, todosPipe, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", kernel.Health(todosRepo))
	todosHandlers.Register(mux)
	handler := kernel.Middleware(logger)(mux)

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
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
