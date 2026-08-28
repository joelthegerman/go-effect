package kernel

import (
	"context"
	"log/slog"
	"net/http"

	"agentic-sandbox/internal/core"
)

// Pipeline is the shared write tail every feature handler uses: GATE then RUN.
// A handler plans effects with its pure core, then calls Apply — the gate cannot
// be skipped. Feature policies are injected once at wiring time.
type Pipeline struct {
	policies []Policy
	uow      UnitOfWork
	log      *slog.Logger
}

// NewPipeline bundles the executor with the feature guardrails it must enforce.
func NewPipeline(uow UnitOfWork, logger *slog.Logger, policies ...Policy) *Pipeline {
	return &Pipeline{policies: policies, uow: uow, log: logger}
}

// Apply gates a planned batch of effects and, if it passes, executes it
// atomically. This is the plan -> GATE -> RUN handoff.
func (p *Pipeline) Apply(ctx context.Context, effects []core.Effect) error {
	v, err := Gate(p.policies, effects)
	if err != nil {
		return err
	}
	return Run(ctx, p.uow, p.log, v)
}

// Pinger is the tiny surface the health check needs from a datastore.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Health returns a handler for GET /healthz that reports database reachability.
func Health(p Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := p.Ping(r.Context()); err != nil {
			WriteError(w, http.StatusServiceUnavailable, "unavailable", "database unreachable")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
