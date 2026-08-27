// Package api is the HTTP layer: routing, request decoding, response encoding,
// and middleware. It translates HTTP into the core -> gate -> shell flow and
// holds no business logic of its own.
package api

import (
	"context"
	"log/slog"
	"net/http"

	"agentic-sandbox/internal/core"
	"agentic-sandbox/internal/shell"
)

// Store is the read surface the API needs from the shell. Writes never go
// through here — they go through the gate via applyEffects — which is why this
// interface exposes only reads (plus Ping for health).
type Store interface {
	List(ctx context.Context, p shell.ListParams) ([]core.Todo, error)
	Get(ctx context.Context, id string) (core.Todo, error)
	Ping(ctx context.Context) error
}

// runner executes a vetted batch of effects. It matches shell.Run and is a
// field so tests can substitute a fake.
type runner func(ctx context.Context, v shell.Vetted) error

type Server struct {
	store Store
	run   runner
	log   *slog.Logger
}

// NewServer wires the API to the shell's repository and executor.
func NewServer(repo *shell.Repo, logger *slog.Logger) *Server {
	return &Server{
		store: repo,
		run:   func(ctx context.Context, v shell.Vetted) error { return shell.Run(ctx, repo, logger, v) },
		log:   logger,
	}
}

// Handler returns the fully wrapped http.Handler (routes + middleware).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /todos", s.handleList)
	mux.HandleFunc("POST /todos", s.handleCreate)
	mux.HandleFunc("GET /todos/{id}", s.handleGet)
	mux.HandleFunc("PATCH /todos/{id}", s.handleUpdate)
	mux.HandleFunc("DELETE /todos/{id}", s.handleDelete)

	return recoverer(s.log)(requestID(requestLogger(s.log)(mux)))
}

// applyEffects is the shared write tail: GATE then RUN. Handlers build effects
// via core, then call this — the gate cannot be skipped.
func (s *Server) applyEffects(ctx context.Context, effects []core.Effect) error {
	vetted, err := shell.Gate(effects)
	if err != nil {
		return err
	}
	return s.run(ctx, vetted)
}
